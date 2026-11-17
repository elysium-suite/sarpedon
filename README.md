# sarpedon (Σαρπηδών)

Simple and very fast [aeacus](https://github.com/elysium-suite/aeacus) endpoint.

> **TitanTurtles fork.** This build reads the `scores` collection, adds the admin
> **Monitor** source-IP drill-down, and a **PT Competition web console** at `/manage`.
> The PT competition feature is a two-repo system — see the next section.

## PT Competitions (requires the `triptolemus` repo)

The authed `/manage` pages let an admin create / edit / set-default / hide / remove
Packet Tracer competitions and review student work, all in the browser. The competition
engine, the student agent, and the `.pka` tooling live in a **separate repo**:

**https://github.com/titanturtles/triptolemus**

- **`levelsvc`** (triptolemus) — the levels gating + deploy/review service. sarpedon's web
  console calls it over localhost. Run it on the **same host** as sarpedon (fronted by
  nginx at `/levels/`), with `pka_tool` installed (e.g. `/opt/levelsvc/pka_tool`) so the
  console can read activity hashes from uploaded `.pka` files.
- **`pt_agent`** (triptolemus) — the student competition app; it pulls competitions from
  `levelsvc` and reports per-item scores to this sarpedon.
- **`pka_tool`** (triptolemus) — decrypts `.pka` activities (used server-side by levelsvc).

Wire the console to levelsvc in `sarpedon.conf`:

```toml
levelsvcToken = "<levelsvc admin token>"   # required: the console authenticates to levelsvc with it
# levelsvcUrl = "http://127.0.0.1:8099"    # optional (this is the default)
```

Creators log in with the `[[admin]]` accounts (add blocks for more people). Creating or
changing a competition writes `[[image]]` blocks into `sarpedon.conf`; a **debounced**
watcher (`sarpedon-restart.path`, ~20s) then restarts sarpedon to load the new scoring
image — the debounce is what keeps the web action from 502‑ing mid‑request.

## Installation

Use these steps for a Linux system with `apt`:

```bash
cd /opt
git clone https://github.com/elysium-suite/sarpedon
cd sarpedon
bash install.sh
```

## Usage

```bash
./sarpedon
```

Example configuration (`sarpedon.conf`):

```toml
event = "My Event" # Event name
password = "s3cr3tP4ssw0rd" # Needed for scoring request encryption
playtime = "6h" # PlayTime limit in format https://godoc.org/time#ParseDuration
enforce = false # (Not supported in aeacus) If enforce is set, images will be sent a kill signal after the playtime limit is reached
timezone = "America/Los_Angeles" # Required for all timestamp conversions, in format https://en.wikipedia.org/wiki/List_of_tz_database_time_zones
discordhook = "https://discord.com/api/webhooks/webhook_id/webhook_token" # Optional, for posting image completions to Discord
timeout = 15 # Optional, web server timeout in seconds (default: 15 seconds). Set to -1 for no timeout
allownewteam = true

[[admin]] # Admin account to view vulnerabilities scored
username = "admin"
password = "mypassword:)"

[[image]]
name = "Linux-Machine" # Image name set in vulnerability remediation engine configuration
color = "#ff00ff" # Optional

[[image]]
name = "Windows-Machine"
color = "#00ff00"

[[team]]
id = "MyId1"
alias = "CoolTeam1"
email = "coolteam1@example.org" # Optional

[[team]]
id = "MyId2"
alias = "CoolTeam2"
email = "coolteam2@example.org"
```

Don't know what to use this with? Try [aeacus](https://github.com/elysium-suite/aeacus).
