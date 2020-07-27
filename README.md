# sarpedon (Σαρπηδών)

Simple and very fast [aeacus](https://github.com/sourque/aeacus) endpoint.

Example configuration (`sarpedon.conf`):

```
event = "My Event"
password = "s3cr3tP4ssw0rd"

[[admin]]
username = "admin"
password = "mypassword:)"

[[image]]
name = "Linux-Machine"
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

Known issues:
- Elapsed time calculation appears to be borked
