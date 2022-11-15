# minedive

## Overview
**minedive** is a P2P privacy conscious distributed search engine embedden in 
a browser extension. 

The source for **minedive-server** isn't located no more in its own repository:
[godive](https://github.com/ckin-it/godive) but it's available in this repo 
in [mined](https://github.com/ckin-it/minedive/tree/master/minedive/cmd/mined)

## Installation (Chrome or Chromium)

**minedive** can be installed through the [Chrome Web Store](https://chrome.google.com/webstore/detail/minedive/cenpmgfnfoimonikmpmjejbnongajbhg). 

## Installattion (Firefox)

**minedive** can be installed through the [Firefox Browser Add-ons website](https://addons.mozilla.org/en-US/firefox/addon/minedive/).

## Installation (from sources, on Chrome and Chromium)

from this repository using the following procedure:

- clone this repository
- build minedive.wasm (with make it will be automatically copied inside the browser extension)
- go to chrome://extensions on your Chrome browser
- turn on developer mode if not already on
- load unpacked extension from the browser\_extension directory

## Documentation
More documentation about design and usage can be found in the 
[Wiki](https://github.com/ckin-it/minedive/wiki)
