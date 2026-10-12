# Go ICS library for listing one day's Meetings (as of 2026-10-12)

Answers [#53](https://github.com/tedkulp/pholio/issues/53) (part of map #52). Sources are the module proxy (`go list -m`, `go get` on 2026-10-12), the module source at the cited version (paths like `golang-ical@v0.3.7/components.go` mean that file in the module cache), GitHub repo metadata (`gh api repos/<owner>/<repo>` on 2026-10-12), and RFC 5545. The throwaway check described in §4 was run on 2026-10-12 with `go1.27.2` and `TZ=America/New_York`.

## Recommendation

**Use `github.com/arran4/golang-ical` to parse, and `github.com/teambition/rrule-go` to expand RRULE/RDATE/EXDATE. Write about 40 lines of our own glue to apply RECURRENCE-ID overrides and clip to the day.** Enable `ics.WithWindowsTimezoneMapping()` when parsing.

- It is the only candidate that gave the right answer for every case in the sample feed (§4).
- golang-ical says plainly that it does not expand recurrences and points to rrule-go plus a RECURRENCE-ID override pattern (`golang-ical@v0.3.7/README.md:171-272`). We would write that glue whatever the parser.
- Neither library pulls anything into the binary beyond the stdlib (§5).
- Drop gocal: its expander is hand-rolled and incomplete, and it is silently wrong on Outlook timezones and on all-day dates. Drop emersion/go-ical: it fails on comma-separated EXDATE and on Windows TZIDs. Don't hand-roll: the RRULE expander is the hard part, and rrule-go already does it.

## 1. Candidates at a glance

| | arran4/golang-ical | emersion/go-ical | apognu/gocal | teambition/rrule-go |
|---|---|---|---|---|
| Version | v0.3.7 (2026-09-30) | untagged, `v0.0.0-20250609112844-439c63cef608` | v0.9.1 (2024-10-24) | v1.8.2 (2023-01-13) |
| Last push | 2026-10-10 | 2025-06-09 | 2024-10-27 | 2024-08-15 |
| Stars / open issues | 417 / 22 | 72 / 5 | 89 / 6 | 383 / 9 |
| License | Apache-2.0 | MIT | MIT | MIT |
| Non-test deps | none | rrule-go | `ChannelMeter/iso8601duration` (2015 pseudo-version) | none |
| RRULE expansion | no; parses only | yes, via rrule-go (`RecurrenceSet`) | own partial expander | yes, full RFC 5545 RRULE + Set |
| RECURRENCE-ID overrides | caller does it (README pattern) | caller does it | built in (`IsRecurringInstanceOverriden`) | n/a |
| Windows TZIDs | `WithWindowsTimezoneMapping()` | error | silently falls back to UTC | n/a |

Version and date come from `go list -m -json <mod>@latest`. Push date, stars, issues and license come from `gh api repos/...`. Dependencies come from each module's `go.mod`.

## 2. Requirement-by-requirement

RFC 5545 references: DTSTAMP is required in VEVENT (§3.6.1); floating time is "DATE-TIME without Z or TZID" (§3.3.5); EXDATE may hold a comma-separated list (§3.8.5.1); RECURRENCE-ID together with UID identifies one occurrence (§3.8.4.4); a date-only DTSTART (`VALUE=DATE`) marks an all-day event (§3.3.4, §3.6.1).

### golang-ical v0.3.7
- **Parsing**: `ParseCalendarWithOptions(r, opts...)` (`calendar.go:906`) and `Calendar.Events()` (`calendar.go:703`).
- **Times**: `GetStartAt`/`GetEndAt` go through `parseTimeValue` (`components.go:314-389`). A `Z` value is UTC. A `TZID` value is loaded via `resolveTimezone`, which runs the optional mapper and then `time.LoadLocation` (`timezones.go:191-216`). A floating value is parsed in `time.Local`, which is the correct reading of §3.3.5 for a viewer's agenda. VTIMEZONE blocks are *not* interpreted: TZIDs must be IANA names or Windows names the mapper knows (`windowsToIANA` table in `timezones.go`). An unknown custom TZID is an error, not a silent shift.
- **All-day**: no flag. Detect it with `DTSTART` param `VALUE=DATE`, or an 8-character value. `GetAllDayStartAt`/`GetAllDayEndAt` exist (`components.go:395`, `:730`).
- **EXDATE**: `GetExDates()` handles both comma lists and repeated properties (`components.go:439-443`).
- **RECURRENCE-ID**: `GetRecurrenceID()` (`components.go:446`). RRULE is exposed both raw (`GetProperty(ComponentPropertyRrule).Value`) and typed (`GetRRules`, `recurrence.go`).
- **ATTENDEE**: `Attendees()` returns `*Attendee` with `Email()` (strips `mailto:`) and `ParticipationStatus()`; read CN from `ICalParameters["CN"]` (`components.go:593-635`). PARTSTAT is ready for the "hide declined" question that #52 left open.
- **Extras**: `ParseCalendarFromUrl` exists (`calendar.go:738`), but pholio should do its own `http.Get` to control timeouts and errors.

### emersion/go-ical + rrule-go
- `Component.RecurrenceSet(loc)` builds an `rrule.Set` from RRULE, EXDATE and RDATE (`components.go:17-55`).
- **Bug for us**: `Prop.DateTime` decides the format from the value's length (`ical.go:124-160`). A comma-separated EXDATE such as `20261013T093000,20261020T093000` then fails to parse, and `RecurrenceSet` returns an error for the whole series (seen in §4). Google and Outlook both emit multi-value EXDATE.
- A TZID goes straight to `time.LoadLocation` with no mapper, so `TZID=Eastern Standard Time` is an error (`ical.go:149-153`).
- Untagged, last commit June 2025.

### apognu/gocal v0.9.1
- Expands recurrences itself in `ExpandRecurringEvent` (`rrule.go:13-149`). It reads only FREQ, UNTIL, COUNT, INTERVAL, a *single* BYMONTH integer, and BYDAY via `strings.Contains`. So ordinals like `2TU` or `-1FR` are ignored, and any matching weekday in the window counts. BYMONTHDAY, BYSETPOS, BYYEARDAY and WKST are not supported. Monthly rules like "last Friday" will produce wrong dates.
- Unknown TZID: tries `TZMapper`, then `LoadTimezone`, then **silently uses UTC** (`parser/time.go:50-63`). In §4, an Outlook 12:00 Meeting showed at 07:00.
- All-day dates are placed in `AllDayEventsTZ`, which is nil (UTC) by default (`parser/time.go:27-38`). A New York user sees the all-day "Holiday" at 19:00 the day before unless the caller sets it.
- Strict by default: a VEVENT without DTSTAMP fails the whole feed (§4). That is RFC-correct but brittle.
- Parses ATTENDEE into `Attendees []Attendee{Cn, Status, Value}` (`types.go:163-169`). RECURRENCE-ID is a raw `string` (`types.go:142`).

### Hand-rolled
Line unfolding, parameter parsing and TZID loading are around 150 lines. RRULE expansion (BYDAY ordinals, BYSETPOS, BYMONTHDAY, COUNT/UNTIL across DST) is the bulk of the risk, and rrule-go already covers it. The part that is left over, applying overrides and clipping to the day, has to be written in every option. That leaves nothing for a hand-rolled parser to save.

### rrule-go v1.8.2
- `StrToROption` parses the RRULE value. `rrule.Set` with `RRule`, `RDate`, `ExDate` and `Between(after, before, inc)` gives the occurrences of a window. Occurrences are generated in DTSTART's `Location`, so a 09:30 America/New_York series stays at 09:30 across the 2026-11-01 DST change (shown in §4: 09:30 on Nov 3 from an October DTSTART).
- Quiet since January 2023, MIT, zero deps. It is a stable leaf: if it ever breaks, vendoring or forking it is cheap.

## 3. Glue pholio still writes (with any parser)

1. Parse each feed and split VEVENTs into masters (no RECURRENCE-ID) and overrides (`UID`, RECURRENCE-ID instant) → event.
2. For a non-recurring master, keep it if `[start, end)` overlaps the day.
3. For a recurring master, build an `rrule.Set` (DTSTART = parsed start, plus EXDATEs and RDATEs), then call `Between(dayStart - duration, dayEnd, true)`. Drop any occurrence whose (UID, instant) has an override.
4. Add the overrides whose own DTSTART/DTEND overlaps the day. Skip `STATUS:CANCELLED`.
5. Mark as all-day any event whose DTSTART is `VALUE=DATE`. #52 says the list hides these.
6. **Per-occurrence identity** = UID + original occurrence start (the RECURRENCE-ID value for an override, the generated instant for a master), formatted in UTC as `YYYYMMDDTHHMMSSZ`. Use the same value for the `meeting_uid:` frontmatter, so a moved override still matches its Meeting Note.

Not handled by any candidate, and skipped: `RANGE=THISANDFUTURE` on RECURRENCE-ID (RFC 5545 §3.8.4.4), and interpreting custom VTIMEZONE definitions whose TZID is neither IANA nor a known Windows name. Add these if a real feed needs them.

## 4. Throwaway check

A sample feed with: a weekly `TZID=America/New_York` standup with a comma EXDATE, two ATTENDEEs (one with CN) and a RECURRENCE-ID override moving the 2026-11-10 occurrence from 09:30 to 14:00; a `FREQ=MONTHLY;BYDAY=2TU` meeting; an all-day `VALUE=DATE` event; a floating-time event; an Outlook-style `TZID=Eastern Standard Time` event with VTIMEZONE; a UTC weekly meeting with a single EXDATE on 2026-11-10. The query day was 2026-11-10, with local time America/New_York. Expected: moved standup 14:00, review 11:00, holiday (all-day), floating 16:00, Outlook 12:00, and no excluded weekly.

| Result | golang-ical + rrule-go | emersion + rrule-go | gocal |
|---|---|---|---|
| Moved standup 14:00, 09:30 suppressed | yes | yes, but only because the master errored | yes |
| Comma EXDATE | ok | **error, series dropped** | ok |
| 2nd-Tuesday review 11:00 | yes | yes | yes (window luck, see §2) |
| All-day holiday | local midnight | local midnight | **19:00 previous day (UTC)** |
| Floating 16:00 | yes | yes | yes |
| Outlook TZ 12:00 | yes (with mapper) | **error** | **07:00 (silent UTC)** |
| Single EXDATE honoured | yes | yes | yes |
| ATTENDEE CN/email | yes | yes | yes |
| Missing DTSTAMP | tolerated | tolerated | **whole feed rejected** |

The check also ran for 2026-11-03: all three put the standup at 09:30 EST from an EDT DTSTART, so DST is handled. The script and sample were scratch files and are not kept in the repo.

## 5. Dependency weight

`go list -deps github.com/arran4/golang-ical github.com/teambition/rrule-go` lists only the stdlib plus those two packages. golang-ical's `go.mod` requires `go-cmp` and `testify`, but only its tests use them. They enter pholio's `go.sum` and module graph, not the binary. golang-ical declares `go 1.20` and rrule-go `go 1.16`, both well below pholio's `go 1.27.1`. Apache-2.0 (golang-ical) and MIT (rrule-go) are both permissive. Apache-2.0 asks us to keep its NOTICE/LICENSE text if we redistribute, and GoReleaser archives don't currently bundle third-party licenses. That is worth a line in the spec, but it doesn't block anything.
