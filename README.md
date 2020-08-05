# sarpedon (Σαρπηδών)

Simple and very fast [aeacus](https://github.com/sourque/aeacus) endpoint.

## Installation

```bash
cd /opt
git clone https://github.com/sourque/sarpedon
bash /opt/sarpedon/misc/install.sh
```

## Usage
```bash
cd /opt/sarpedon
go build # Builds sarpedon binary
touch sarpedon.conf
./sarpedon # After you finish your sarpedon.conf
```

Example configuration (`sarpedon.conf`):

```toml
event = "My Event" # Event name
password = "s3cr3tP4ssw0rd" # Needed for scoring request encryption

[[admin]] # Admin account to view vulnerabilities scored
username = "admin"
password = "mypassword:)"

[[image]]
name = "Linux-Machine" # Image name set in Aeacus engine configuration
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

## Known Issues
- Elapsed time calculation appears to be borked
