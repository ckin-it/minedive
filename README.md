# minedive

## Overview
**minedive** is a P2P privacy conscious distributed search engine embedden in 
a browser extension. 

The source for **minedive-server** isn't located no more in its own repository:
[godive](https://github.com/ckin-it/godive) but it's available in this repo 
in [mined](https://github.com/ckin-it/minedive/tree/master/minedive/cmd/mined)

## Installation
**minedive** can be installed through the Chrome and Firefox Web Stores:

- [chrome web store](https://chrome.google.com/webstore/detail/minedive/cenpmgfnfoimonikmpmjejbnongajbhg)
- [firefox web store](https://addons.mozilla.org/en-US/firefox/addon/minedive/)

or from this repository using the following procedure:

- clone this repository
- build it
- go to chrome://extensions on your Chrome browser
- turn on developer mode if not already on
- load unpacked extension from the repository folder

## Documentation
More documentation about design and usage can be found in the 
[Wiki](https://github.com/ckin-it/minedive/wiki)

## Building the extension

You need go 1.17 installed.

```
$ git clone https://github.com/ckin-it/minedive.git
$ go get -u
$ cd minedive/minedive
$ make
```

This will create the minedive.wasm file.

