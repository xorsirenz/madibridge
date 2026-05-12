# madibridge
<code>matrix-discord bridge.. its bad so dont use(ever)</code>

## Requirements:
- \>= [golang v1.26.2](https://go.dev/)
- \>= [postgreSQL v11](https://www.postgresql.org)

### Discord bot requirements:
#### Intents:
- Message Content Intent
#### OAuth2 Scope:
##### - bot
    - View Channels
    - Send Messages
    - Manage Messages
    - Embed Links
    - Attach Files
    - Send Messages in Threads
    - Read Message History
    - Add Reactions
    - Manage Webhooks

## Installation:
```sh
$ git clone https://github.com/xorsirenz/madibridge
$ cd madibridge

# copy sample config to the root directory
$ cp config/madibridge-sample.yaml madibridge.yaml
```
#### Setting up PostgreSQL:
```sh
# create a postgreSQL user
# postgreSQL will prompt a pass word for the new user
$ sudo -u postgres createuser -P madibridge

# create a postgreSQL database madibridge with the owner as madibridge
$ sudo -u postgres createdb -O madibridge madibridge
```

#### Build and run:
```sh
# build
$ make

# usage
$ ./madibridge
```
