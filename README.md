# pds-discord-auth

because i'm NOT going to setup smtp on my pds, nuh uh!!!!

this is a little tool that maps dids to discord user ids and scrapes the `email_token`
table in bsky pds to then send the tokens over discord dms to users

## how

```sh
# build the thing
git clone ...
go build -o pds-discord-auth

# send it somewhere

# now on somewhere
export DISCORD_TOKEN=your_bot_token_here
export DATABASE_PATH=/blah/blah/blah/bot.db
export PDS_ACCOUNT_SQLITE_PATH=/path/to/your/pds/datadir/account.sqlite
./pds-discord-auth adduser <did> <discord user id>

# then start the bot (daemon somewhere, systemd whatever)
./pds-discord-auth run
```
