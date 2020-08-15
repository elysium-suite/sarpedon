# sarpedon (Σαρπηδών)

Simple and very fast [aeacus](https://github.com/sourque/aeacus) endpoint.

## Installation

```bash
cd /opt
git clone https://github.com/sourque/sarpedon
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

Don't know what to use this with? Try [aeacus](https://github.com/sourque/aeacus).
