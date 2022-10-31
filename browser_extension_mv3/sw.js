import gianfranco from './wasm_exec.js';

//BEGIN core/conf.js
const default_options = {
  staticID: "",
  minL1: 1,
  minL2: 2,
  stun: [{urls: "stun:stun.l.google.com:19302"}],
  ws_server: "wss://godive.ckin.it/ws",
  community: "",
  lang: navigator.language || navigator.userLanguage
};
//END core/conf.js

let browser = chrome || browser;
let log = console.log;

let searchPorts = [];

function sbbc() {
  log("invoked?");
  browser.action.setBadgeText({text:"1"});
}

browser.runtime.onConnect.addListener(function(port) {
  port.onMessage.addListener( function(msg) {
    log("port print:", msg);
    switch(msg.type) {
      case 'log':
        log(msg.text);
      case 'status':
        port.postMessage({type: "status", text: "status_log", l1: "peers_number_l1", l2: "peers_number_l2"});
        break;
      case 'search':
        if (!msg.q) {
          log("this port is a search page");
          searchPorts.push(port);
          log("search ports:", searchPorts.length);
        } else {
          log('searching for ['+msg.q+']');
        }
      break;
    }
  });
});

function respond(a) {
  log('respond invoked', a);
  for( var i = 0; i < searchPorts.length; i++){ 
    try { 
      log('send to...', i, searchPorts[i]);
      searchPorts[i].postMessage(a); 
    } catch {
      log('splice connectedPort', i)
      searchPorts.splice(i, 1);
      i--;
    }
  }
}


gianfranco();
setTimeout(sbbc, 3 * 1000);
//const go = new global.Go();
async function init(options) {
  let w = new Window();
  let peerConn = new w.RTCPeerConnection(); 
  //let result = await WebAssembly.instantiateStreaming(fetch("minedive.wasm"), go.importObject)
  //go.run(result.instance);
  //MinediveConnect(options.ws_server, options.minL1, options.minL2);
  log("init done");
}


chrome.storage.local.get(['options'], function(result) {
  let options;
  if(result['options']) options = result['options'];
  else options = default_options;
  init(options);
});
