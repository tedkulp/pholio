package calendar

import (
	"net/url"
	"regexp"
	"strings"

	ics "github.com/arran4/golang-ical"
)

// details fills m's Attendees, Location, Link and Description from ev.
// golang-ical has already ICS-unescaped the text values.
func (m *Meeting) details(ev *ics.VEvent) {
	m.Location = text(ev, "LOCATION")
	if m.Location == "" {
		m.Location = text(ev, "X-MICROSOFT-LOCATIONDISPLAYNAME")
	}
	m.Description = text(ev, "DESCRIPTION")
	m.Attendees = attendees(ev)
	m.Link = link(ev, m.Location, m.Description)
}

func text(ev *ics.VEvent, name string) string {
	if p := ev.GetProperty(ics.ComponentProperty(name)); p != nil {
		return strings.TrimSpace(p.Value)
	}
	return ""
}

// attendees is the ATTENDEEs in feed order, CN else email, deduped by
// email, without rooms, resources and those who declined.
func attendees(ev *ics.VEvent) []string {
	var out []string
	seen := map[string]bool{}
	for _, a := range ev.Attendees() {
		cutype := strings.ToUpper(first(a.ICalParameters["CUTYPE"]))
		if cutype == "ROOM" || cutype == "RESOURCE" || strings.EqualFold(string(a.ParticipationStatus()), "DECLINED") {
			continue
		}
		email := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(a.Value, "mailto:"), "MAILTO:"))
		key := strings.ToLower(email)
		if seen[key] {
			continue
		}
		seen[key] = true
		if name := strings.TrimSpace(first(a.ICalParameters["CN"])); name != "" {
			out = append(out, name)
		} else if email != "" {
			out = append(out, email)
		}
	}
	return out
}

func first(vs []string) string {
	if len(vs) == 0 {
		return ""
	}
	return vs[0]
}

// linkProps are read in order; the first http(s) value is the link.
var linkProps = []string{
	"X-GOOGLE-CONFERENCE",
	"X-MICROSOFT-SKYPETEAMSMEETINGURL",
	"X-MICROSOFT-ONLINEMEETINGEXTERNALLINK",
	"X-MICROSOFT-ONLINEMEETINGCONFLINK",
	"URL",
}

// callHosts are the hosts (and their subdomains) whose URLs count as a
// call link when found in LOCATION or DESCRIPTION.
var callHosts = []string{"zoom.us", "meet.google.com", "teams.microsoft.com", "teams.live.com", "webex.com"}

var urlRE = regexp.MustCompile(`(?i)https?://[^\s<>"']+`)

// link is the Meeting's call link: the first http(s) linkProps value, else
// the first call-host URL in location, then description.
func link(ev *ics.VEvent, location, description string) string {
	for _, name := range linkProps {
		if v := text(ev, name); isHTTP(v) {
			return v
		}
	}
	for _, s := range []string{location, description} {
		for _, raw := range urlRE.FindAllString(s, -1) {
			raw = strings.TrimRight(raw, ".,;:!?)]")
			if u, err := url.Parse(raw); err == nil && callHost(u.Hostname()) {
				return raw
			}
		}
	}
	return ""
}

func isHTTP(v string) bool {
	u, err := url.Parse(v)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func callHost(host string) bool {
	host = strings.ToLower(host)
	for _, h := range callHosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return true
		}
	}
	return false
}
