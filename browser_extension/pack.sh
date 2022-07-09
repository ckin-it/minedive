#!/bin/bash

cp core/conf.js-RELEASE core/conf.js
zip -r -FS ../minedive.zip * --exclude *.git* pack.sh README.md core/conf.js-RELEASE core/conf.js-GITHUB manifest.json-BACKUP
cp core/conf.js-GITHUB core/conf.js
