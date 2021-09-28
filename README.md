# Twitch Safety Bot

A bot to run in your chat that will help keep you safe from hate raids.

Features:

- Reads a list of patterns; and bans & blocks:
  - any viewers matching those patterns
  - any new followers or viewers matching those patterns
  - any existing followers matching those patterns
- Handles rate limiting for you
- Allows you to add new patterns on the fly
- Is run locally so you know what's happening
- Produces a "per session" ban list to share around

Expected filestructure:
.
├── filters
├── lists
└── logs

Filters: where files containing one regular expression per line are stored,
can be as many or as few files as possible. Warning: it will try to parse
every line of every file in this folder.

Lists: where files containing one username per line are stored. These usernames
will be treated as complete, and blocks and bans will be attempted against them.
Warning: it will try to parse every line of every file in this folder.

Logs: where the bot will write logs of new bans it has performed. Each session
will be a unique log. The name for each session will be a timestamp.
