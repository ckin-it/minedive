window.browser = window.browser || window.chrome || window.msBrowser;
let bkg = browser.extension.getBackgroundPage();
let log = bkg.console.log;

let port = browser.runtime.connect();

window.onload = function() {
  port.postMessage({type: 'get_options'});
}

port.onMessage.addListener(function(msg) {
  switch(msg.type) {
    case 'options':
      load_options(msg.options);
      break;
  }
});

function buttons_disabled(disabled) {
  document.getElementById('apply').disabled = disabled;
  document.getElementById('restore').disabled = disabled;
}

function load_options(o) {
  document.getElementById('minL1').value = o.minL1;
  document.getElementById('minL2').value = o.minL2;
  document.getElementById('ws_server').value = o.ws_server;
  document.getElementById('stun').value = JSON.stringify(o.stun);
  document.getElementById('staticID').value = o.staticID;
  document.getElementById('community').value = o.community;
  buttons_disabled(false);
}

function restore_defaults() {
  chrome.storage.local.remove(['options'], function () {
    port.postMessage({type: 'get_options'});
    port.postMessage({type: 'apply_options'});
  });
}

function save_apply_options() {
  _save_options(function (){
    port.postMessage({type: 'apply_options'});
    port.postMessage({type: 'get_options'});
  });
}

function _save_options(cb) {
  let o = {};
  //XXX check input
  o.minL1 = document.getElementById('minL1').value;
  o.minL2 = document.getElementById('minL2').value;
  o.ws_server = document.getElementById('ws_server').value;
  o.stun = JSON.parse(document.getElementById('stun').value);
  o.staticID = document.getElementById('staticID').value;
  o.community = document.getElementById('community').value;
  buttons_disabled(true);
  chrome.storage.local.set({['options']: o}, cb);
}

function apply_options() {
  port.postMessage({type: 'apply_options'});
}

document.getElementById('apply').onclick = save_apply_options;
document.getElementById('restore').onclick = restore_defaults;
buttons_disabled(true);
