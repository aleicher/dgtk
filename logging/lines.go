package logging

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type SyslogLine struct {
	Raw        string
	Time       time.Time
	Host       string
	Tag        string
	Severity   string
	Pid        int
	fields     []string
	parsed     bool
	tags       map[string]interface{}
	tagsParsed bool
}

var validKeyRegexp = regexp.MustCompile("(?i)^[a-z]+$")
var callsRegexp = regexp.MustCompile("^([0-9.]+)\\/([0-9]+)$")

func removeQuotes(raw string) string {
	if strings.HasPrefix(raw, `"`) && strings.HasSuffix(raw, `"`) {
		return raw[1 : len(raw)-1]
	}
	return raw
}

func parseTags(raw string) map[string]interface{} {
	fields := strings.Fields(raw)
	inQuotes := false
	currentKey := ""
	valueParts := []string{}
	t := map[string]interface{}{}
	for _, field := range fields {
		if inQuotes {
			valueParts = append(valueParts, field)
			if strings.Contains(field, `"`) {
				inQuotes = false
				v := strings.Join(valueParts, " ")
				t[currentKey] = removeQuotes(v)
			}
		} else {
			kv := strings.SplitN(field, "=", 2)
			if len(kv) == 2 && validKeyRegexp.MatchString(kv[0]) {
				currentKey = kv[0]
				value := kv[1]
				if strings.Contains(value, `"`) && !strings.HasSuffix(value, `"`) {
					valueParts = []string{value}
					inQuotes = true
				} else if value != "-" {
					m := callsRegexp.FindStringSubmatch(value)
					if len(m) == 3 {
						totalTime, e := strconv.ParseFloat(m[1], 64)
						if e == nil {
							calls, e := strconv.ParseInt(m[2], 10, 64)
							if e == nil {
								t[currentKey+"_time"] = totalTime
								t[currentKey+"_calls"] = calls
							}
						}
					} else {
						t[currentKey] = parseTagValue(value)
						currentKey = ""
					}
				}
			}
		}
	}
	return t
}

func (line *SyslogLine) Tags() (t map[string]interface{}) {
	if !line.tagsParsed {
		line.tags = parseTags(line.Raw)
	}
	return line.tags
}

func parseTagValue(raw string) interface{} {
	if i, e := strconv.ParseInt(raw, 10, 64); e == nil {
		return i
	} else if f, e := strconv.ParseFloat(raw, 64); e == nil {
		return f
	}
	return removeQuotes(raw)
}

const (
	timeLayout             = "2006-01-02T15:04:05.000000-07:00"
	timeLayoutWithoutMicro = "2006-01-02T15:04:05-07:00"
)

var TagRegexp = regexp.MustCompile("(.*?)\\[(\\d*)\\]")

func (line *SyslogLine) Parse(raw string) (e error) {
	if line.parsed {
		return nil
	}
	line.Raw = raw
	line.fields = strings.Fields(raw)
	if len(line.fields) >= 3 {
		line.Time, e = time.Parse(timeLayout, line.fields[0])
		if e != nil {
			line.Time, e = time.Parse(timeLayoutWithoutMicro, line.fields[0])
			if e != nil {
				return e
			}
		}
		line.Host = line.fields[1]
		line.Tag, line.Severity, line.Pid = parseTag(line.fields[2])
	}
	line.parsed = true
	return nil
}

func parseTag(raw string) (tag, severity string, pid int) {
	tagAndSeverity, pid := splitTagAndPid(raw)
	tag, severity = splitTagAndSeverity(tagAndSeverity)
	return tag, severity, pid
}

func splitTagAndSeverity(raw string) (tag, severity string) {
	tag = raw
	parts := strings.Split(raw, ".")
	if len(parts) == 2 {
		tag, severity = parts[0], parts[1]
	} else {
		tag = raw
	}
	return tag, severity
}

func splitTagAndPid(raw string) (tag string, pid int) {
	tag = raw
	chunks := TagRegexp.FindStringSubmatch(raw)
	if len(chunks) > 2 {
		tag = chunks[1]
		pid, _ = strconv.Atoi(chunks[2])
	} else {
		if tag[len(tag)-1] == ':' {
			tag = tag[0 : len(tag)-1]
		}
	}
	return tag, pid
}

var UUIDRegexp = regexp.MustCompile("([a-z0-9\\-]{36})")

type UnicornLine struct {
	UUID string
	SyslogLine
}

func (line *UnicornLine) Parse(raw string) error {
	e := line.SyslogLine.Parse(raw)
	if e != nil {
		return e
	}
	if line.Tag != "unicorn" {
		return fmt.Errorf("tag %q not supported", line.Tag)
	}
	if len(line.fields) >= 4 {
		parts := UUIDRegexp.FindStringSubmatch(raw)
		if len(parts) > 1 {
			line.UUID = parts[1]
		}
	}
	return nil
}

type NginxLine struct {
	*SyslogLine
	Method        string
	Status        string
	Length        int
	TotalTime     float64
	UnicornTime   float64
	HttpHost      string
	UserAgentName string
	Uri           string
	Referer       string
}

var quotesRegexp = regexp.MustCompile(`(ua|uri|ref)="(.*?)"`)

func (line *NginxLine) Parse(raw string) error {
	if line.SyslogLine == nil {
		line.SyslogLine = &SyslogLine{}
	}
	e := line.SyslogLine.Parse(raw)
	if e != nil {
		return e
	}
	if line.Tag != "ssl_endpoint" && line.Tag != "nginx" {
		return fmt.Errorf("tag %q not supported", line.Tag)
	}
	for _, field := range line.fields {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) == 2 {
			key := parts[0]
			value := parts[1]
			switch key {
			case "method":
				line.Method = value
			case "status":
				line.Status = value
			case "host":
				line.HttpHost = value
			case "length":
				line.Length, _ = strconv.Atoi(value)
			case "total":
				line.TotalTime, _ = strconv.ParseFloat(value, 64)
			case "unicorn_time":
				line.UnicornTime, _ = strconv.ParseFloat(value, 64)
			}
		}
	}
	quotes := quotesRegexp.FindAllStringSubmatch(raw, -1)
	for _, quote := range quotes {
		switch quote[1] {
		case "ua":
			line.UserAgentName = quote[2]
		case "uri":
			line.Uri = quote[2]
		case "ref":
			line.Referer = quote[2]
		default:
		}
	}
	return nil
}

type HAProxyLine struct {
	SyslogLine
	Frontend            string
	Backend             string
	BackendHost         string
	BackendImageId      string
	BackendContainerId  string
	Status              string
	Length              int
	ClientRequestTime   int
	ConnectionQueueTime int
	TcpConnectTime      int
	ServerResponseTime  int
	SessionDurationTime int
	ActiveConnections   int
	FrontendConnections int
	BackendConnectons   int
	ServerConnections   int
	Retries             int
	ServerQueue         int
	BackendQueue        int
	Method              string
	Uri                 string
}

func (line *HAProxyLine) Parse(raw string) error {
	e := line.SyslogLine.Parse(raw)
	if e != nil {
		return e
	}
	if line.Tag != "haproxy" {
		return fmt.Errorf("tag was %s", line.Tag)
	}
	if len(line.fields) > 16 {
		line.Frontend = line.fields[5]
		backend := line.fields[6]
		parts := strings.SplitN(backend, "/", 2)
		if len(parts) == 2 {
			line.Backend = parts[0]
			backendContainer := parts[1]
			parts := strings.Split(backendContainer, ":")
			if len(parts) == 3 {
				line.BackendHost = parts[0]
				line.BackendImageId = parts[1]
				line.BackendContainerId = parts[2]
			}
		}
		times := line.fields[7]
		parts = strings.Split(times, "/")
		if len(parts) == 5 {
			line.ClientRequestTime, _ = strconv.Atoi(parts[0])
			line.ConnectionQueueTime, _ = strconv.Atoi(parts[1])
			line.TcpConnectTime, _ = strconv.Atoi(parts[2])
			line.ServerResponseTime, _ = strconv.Atoi(parts[3])
			line.SessionDurationTime, _ = strconv.Atoi(parts[4])
		}
		line.Status = line.fields[8]
		line.Length, _ = strconv.Atoi(line.fields[9])

		connections := line.fields[13]
		parts = strings.Split(connections, "/")
		if len(parts) == 5 {
			line.ActiveConnections, _ = strconv.Atoi(parts[0])
			line.FrontendConnections, _ = strconv.Atoi(parts[1])
			line.BackendConnectons, _ = strconv.Atoi(parts[2])
			line.ServerConnections, _ = strconv.Atoi(parts[3])
			line.Retries, _ = strconv.Atoi(parts[4])
		}

		queues := line.fields[14]
		parts = strings.Split(queues, "/")
		if len(parts) == 2 {
			line.ServerQueue, _ = strconv.Atoi(parts[0])
			line.BackendQueue, _ = strconv.Atoi(parts[1])
		}
		line.Method = line.fields[15][1:]
		line.Uri = line.fields[16]
	}
	return nil
}
