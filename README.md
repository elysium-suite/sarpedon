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
color = "rgb(0,213,3)" # Optional

[[image]]
name = "Windows-Machine"
color = "rgb(123,213,222)"

[[team]]
id = "MyId1"
alias = "CoolTeam1"
email = "coolteam1@example.org" # Optional

[[team]]
id = "MyId2"
alias = "CoolTeam2"
email = "coolteam2@example.org"
```
