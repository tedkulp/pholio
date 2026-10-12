# Meetings come from private ICS feed URLs

pholio reads Meetings from one or more private ICS feed URLs (the "secret address in iCal format" that Google, Outlook/M365 and Fastmail all publish), fetched with a plain read-only HTTP GET. We chose this over the Google Calendar API, Microsoft Graph, macOS EventKit and CalDAV because pholio only reads Meetings, never changes them, and a feed URL gives that with no OAuth app registration, token refresh, platform-specific code or per-provider client. One parser (golang-ical plus rrule-go for recurrence) covers every provider, and the user's real calendars all reach pholio through Fastmail as Outlook-flavoured ICS anyway.

## Consequences

- Feed URLs are secrets, so they live only in the user config (`[[calendars]]`), never in the Vault config, which may be synced or shared. The cached feed bodies are written with mode `0600`.
- Feeds are whole calendars, not a window: pholio fetches each one in full (about 250 KB total for the user's four) and filters to the day it shows.
- pholio cannot create, edit or RSVP to Meetings, and it cannot see anything a provider leaves out of its published feed (feeds rarely carry the user's own declines, for example).
- Freshness depends on the provider's publishing delay as well as pholio's own refetch rule; a just-added Meeting may not show for a while.
- A feed with an unusual dialect (Outlook's `Customized Time Zone`, conference links in `X-` properties) is handled in pholio's own glue rather than by a provider SDK.
