let log = console.log;
//let log = function() {return;}

function restore_default_options() {
  options = default_options;
}

let options = {};

let l1_peers = {};
let l2_peers = {};
let alias_to_peers = {};

let status_log = '';

const keyPair = nacl.box.keyPair();
const publicKey = keyPair.publicKey;
const secretKey = keyPair.secretKey;
const str_publicKey = nacl.util.encodeBase64(publicKey);

let myUsername = null;
let connection = null;


function get_peers_if_needed(){
  let needed = 0;
  if(Object.keys(l1_peers).length < options.minL1)  {
    needed++;
    ws_get_peers();
  }
  if(Object.keys(l2_peers).length < options.minL2) {
    needed++;
    ask_l2_all();
  }
  if(needed) {
    baSetYellow(' ');
  } else {
    baSetGreen(' ');
  }
}

chrome.runtime.onConnect.addListener(function(port) {
  port.onMessage.addListener( function(msg) {
    switch(msg.type) {
      case 'status':
        let peers_number_l1 = Object.keys(l1_peers).length;
        let peers_number_l2 = Object.keys(l2_peers).length;
        port.postMessage({type: "status", text: status_log, l1: peers_number_l1, l2: peers_number_l2});
        break;
      case 'get_options':
        console.log('get_options received');
        chrome.storage.local.get(['options'], function(result) {
          let nmsg = {};
          log(result['options']);
          if(result['options']) nmsg.options = result['options'];
          else nmsg.options = default_options;
          nmsg.type = "options";
          port.postMessage(nmsg);
        });
        break;
      case 'apply_options':
        apply_options();
        break;
      case 'search':
        //XXX currently sending search to all L2 peers
        for(k in l2_peers) {
          let p = l2_peers[k];
          if(p.gw.dc.readyState == 'open')
          {
            let lang = navigator.language || navigator.userLanguage;
            let m = {
              type: 'search',
              l: lang,
              q: msg.q
            };
            sendL2JSON(p, m);
          }
          //XXX sending search_status / err
          //port.postMessage({type: "search_status", data: XXX});
        };
      break;
    }
  });
});

function reconnect() {
  log('reconnecting...');
  l2_peers = {};
  for(let k in l1_peers) {
    let p = l1_peers[k];
    delete_l1_peer(p);
  };
  connection.close();
  ws_connect();
}

// entry points
chrome.storage.local.get(['options'], function(result) {
  if(result['options']) options = result['options'];
  else options = default_options;
  ws_connect();
});

setInterval(get_peers_if_needed, 10000);
setInterval(lazy_ping, 10000);
setInterval(ws_ping, 10000);
