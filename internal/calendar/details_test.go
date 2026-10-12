package calendar_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/tedkulp/pholio/internal/calendar"
)

// meeting parses one VEVENT made of the given extra lines.
func meeting(t *testing.T, lines ...string) calendar.Meeting {
	t.Helper()
	body := strings.Join(append(append([]string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//pholio//synthetic//EN",
		"BEGIN:VEVENT", "UID:m@example.test", "DTSTAMP:20261001T000000Z",
		"DTSTART:20261012T140000Z", "DTEND:20261012T150000Z", "SUMMARY:Sync",
	}, lines...), "END:VEVENT", "END:VCALENDAR"), "\r\n")
	ms, err := calendar.Parse([]byte(body))
	if err != nil || len(ms) != 1 {
		t.Fatalf("Parse = %d Meetings, %v", len(ms), err)
	}
	return ms[0]
}

func TestLink(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{"none", nil, ""},
		{"Google conference first", []string{
			"URL:https://example.test/event",
			"X-MICROSOFT-SKYPETEAMSMEETINGURL:https://teams.microsoft.com/l/b",
			"X-GOOGLE-CONFERENCE:https://meet.google.com/abc-defg-hij",
		}, "https://meet.google.com/abc-defg-hij"},
		{"Teams URL before external link", []string{
			"X-MICROSOFT-ONLINEMEETINGEXTERNALLINK:https://example.test/ext",
			"X-MICROSOFT-SKYPETEAMSMEETINGURL:https://teams.microsoft.com/l/b",
		}, "https://teams.microsoft.com/l/b"},
		{"external link before conf link", []string{
			"X-MICROSOFT-ONLINEMEETINGCONFLINK:https://example.test/conf",
			"X-MICROSOFT-ONLINEMEETINGEXTERNALLINK:https://example.test/ext",
		}, "https://example.test/ext"},
		{"conf link before URL", []string{
			"URL:https://example.test/event",
			"X-MICROSOFT-ONLINEMEETINGCONFLINK:https://example.test/conf",
		}, "https://example.test/conf"},
		{"URL before location", []string{
			"LOCATION:https://zoom.us/j/1",
			"URL:https://example.test/event",
		}, "https://example.test/event"},
		{"non-http skipped", []string{
			"X-GOOGLE-CONFERENCE:tel:+1-555-0100",
			"X-MICROSOFT-ONLINEMEETINGCONFLINK:conf:sip:someone@example.test",
			"URL:https://example.test/event",
		}, "https://example.test/event"},
		{"location before description", []string{
			"DESCRIPTION:Join https://meet.google.com/zzz-zzzz-zzz",
			"LOCATION:Room 4 / https://zoom.us/j/123?pwd=x",
		}, "https://zoom.us/j/123?pwd=x"},
		{"description when location has none", []string{
			"LOCATION:Room 4",
			`DESCRIPTION:Agenda\nJoin: https://teams.microsoft.com/l/meetup-join/1 now`,
		}, "https://teams.microsoft.com/l/meetup-join/1"},
		{"subdomain matches", []string{
			"DESCRIPTION:https://example.test/doc then https://acme.zoom.us/j/9",
		}, "https://acme.zoom.us/j/9"},
		{"lookalike host skipped", []string{
			"DESCRIPTION:https://notzoom.us/j/1 https://zoom.us.example.test/j/2 http://acme.webex.com/m",
		}, "http://acme.webex.com/m"},
		{"teams.live.com", []string{"LOCATION:https://teams.live.com/meet/1"}, "https://teams.live.com/meet/1"},
		{"unknown host in description skipped", []string{"DESCRIPTION:see https://example.test/doc"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := meeting(t, tt.lines...).Link; got != tt.want {
				t.Errorf("Link = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAttendees(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  []string
	}{
		{"none", nil, nil},
		{"CN else email, feed order", []string{
			"ATTENDEE;CN=Ada Example:mailto:ada@example.test",
			"ATTENDEE:mailto:bob@example.test",
			`ATTENDEE;CN="Cole, C.":mailto:cole@example.test`,
		}, []string{"Ada Example", "bob@example.test", "Cole, C."}},
		{"deduped by email", []string{
			"ATTENDEE;CN=Ada Example:mailto:ada@example.test",
			"ATTENDEE;CN=Ada Again:mailto:ADA@example.test",
		}, []string{"Ada Example"}},
		{"rooms, resources and declined dropped", []string{
			"ATTENDEE;CUTYPE=ROOM;CN=Room 4:mailto:room4@example.test",
			"ATTENDEE;CUTYPE=RESOURCE;CN=Projector:mailto:proj@example.test",
			"ATTENDEE;PARTSTAT=DECLINED;CN=Dee:mailto:dee@example.test",
			"ATTENDEE;PARTSTAT=ACCEPTED;CUTYPE=INDIVIDUAL;CN=Eve:mailto:eve@example.test",
		}, []string{"Eve"}},
		{"organizer not added", []string{
			"ORGANIZER;CN=Olga:mailto:olga@example.test",
			"ATTENDEE;CN=Ada Example:mailto:ada@example.test",
		}, []string{"Ada Example"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := meeting(t, tt.lines...).Attendees; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Attendees = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLocationAndDescription(t *testing.T) {
	tests := []struct {
		name                  string
		lines                 []string
		location, description string
	}{
		{"none", nil, "", ""},
		{"unescaped and trimmed", []string{
			`LOCATION:  Room 4\, Floor 2  `,
			`DESCRIPTION:Agenda\n- one\; two  `,
			"X-ALT-DESC;FMTTYPE=text/html:<b>ignored</b>",
		}, "Room 4, Floor 2", "Agenda\n- one; two"},
		{"Microsoft display name when location empty", []string{
			"LOCATION:",
			"X-MICROSOFT-LOCATIONDISPLAYNAME:Room 9",
		}, "Room 9", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := meeting(t, tt.lines...)
			if m.Location != tt.location || m.Description != tt.description {
				t.Errorf("Location, Description = %q, %q; want %q, %q", m.Location, m.Description, tt.location, tt.description)
			}
		})
	}
}
