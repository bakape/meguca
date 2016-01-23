[![GoDoc](https://godoc.org/gopkg.in/bakape/meguca.v2?status.svg)](https://godoc.org/gopkg.in/bakape/meguca.v2) [![Build Status](https://travis-ci.org/bakape/meguca.svg?branch=v2)](https://travis-ci.org/bakape/meguca) [![Dependency Status](https://david-dm.org/bakape/meguca.svg)](https://david-dm.org/bakape/meguca)

##Setup
* Install [dependencies](#dependencies) listed below
* `go get -u gopkg.in/bakape/meguca.v2`
* `meguca.v2 init`
* Configure the server, installed in the standard location of your Go workspace
(configuration WebUI soon™)
* See `meguca.v2 help` for usage guide

##Dependencies
* [Go](https://golang.org/doc/install)
* [RethinkDB](https://rethinkdb.com/docs/install/)
    * RethinkDB does not enable a configuration file by default. If you don't
    want to configure anything, just copy `/etc/rethinkdb/default.conf.sample`
	into `/etc/rethinkdb/instances.d/instance1.conf`. You might also set it to
	autostart on boot. See the [official guide](http://www.rethinkdb.com/docs/start-on-startup/).

##Updating
* `go get -u gopkg.in/bakape/meguca.v2`
* `meguca.v2 restart`
* See `docs/` for upgrading between semver major releases

##Development
* Install [Node.js](https://nodejs.org/en/) >=4.0.0
* `npm install` to install build dependencies
* `npm run-script build` to build the client
* `npm run-script watch` to watch for file changes and automatically
incrementally rebuild the client
* Use `meguca.v2 debug` to run the server

// TODO: Rewrite everything below for v2

##Cross-platform development with Vagrant
* Install [VirtualBox](https://www.virtualbox.org/wiki/Downloads) and
[Vagrant](http://www.vagrantup.com/downloads.html)
* Open a shell in meguca's root directory and run `vagrant up`
* Grab a coffee
* Run `vagrant ssh` and `node builder`, once logged in. Your changes will
automatically sync both ways. [More info](https://www.vagrantup.com/)

##Automatic deployment
Users less familiar with Linux and sysadmin procedures can paste the
following line into the root shell to automatically setup a default
installation on an fresh Ubuntu >=14.04 LTS VPS.
`wget -q -O - https://raw.githubusercontent.com/bakape/meguca/master/scripts/deploy.sh | bash -`

##Production
* Have your webserver serve `www/`
  * It is highly recommended to use a dedicated webserver for serving static
  files and as a reverse proxy. Even if you choose to use the default inbuilt
  webserver, you still need to set `MEDIA_URL` in `config/imager` for image
  search links to work.
* Run `npm start/stop/restart` to start/stop/restart the server
* You can update `config/hot.js` and client files without restarting the server
with `node server/kill.js`
* All errors are logged to `./error.log`

## Documentation
* docs/api.md - JSON API spec
* docs/dev_guide.md - Brief description on project operation for developers
* docs/migration_*.js - Major semvser migration instructions
* docs/doushio.initscript.example - Init script example
* docs/nginx.config.example - ngingx configuration example

##Standalone upkeep scripts
* scripts/backup.js - uploads rdb to S3
* scripts/backup.sh - MEGA backup script
* scripts/purge_bans.sh - Removes all bans
* scripts/purge_mod_sessions.sh - Closes all active moderator sessions
* scripts/radio.js - icecast2 server integration
* scripts/send.js - global websocket push messages
