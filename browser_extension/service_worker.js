let connectedPorts = [];
let result;
let options;

chrome.runtime.onConnect.addListener(function(port) {
  connectedPorts.push(port);
  port.onMessage.addListener( function(msg) {
    switch(msg.type) {
      case 'status':
        if (!restartGoIfExited()) {          
          let minediveState = MinediveGetState(); //XXX fix this
          let circuitState = MinediveGetCircuitState();
          port.postMessage({type: "status", text: minediveState, l1: circuitState, l2: ""});
        }
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
        MinediveReConnect(options.ws_server, options.minL1);
        break;
      case 'new-circuit':
        MinediveNewCircuit(options.ws_server, options.minL1, options.minL2);
        //MinediveReConnect(options.ws_server, options.minL1);
        break;
      case 'search':
        //XXX maybe promise and reply in port?
        //maybe other message polling?
        //maybe storage?
        log('searching for ['+msg.q+']');
        MinediveSearch(msg.q, options.lang);
      break;
    }
  });
});

function respond(a) {
  log('respond invoked', a);
  for( var i = 0; i < connectedPorts.length; i++){ 
    try { 
      log('send to...', i, connectedPorts[i]);
      connectedPorts[i].postMessage(a); 
    } catch {
      log('splice connectedPort', i)
      connectedPorts.splice(i, 1);
      i--;
    }
  }
}

const go = new Go();

async function init(options) {
  result = await WebAssembly.instantiateStreaming(fetch("minedive.wasm"), go.importObject)
  go.run(result.instance);
  MinediveConnect(options.ws_server, options.minL1, options.minL2);
  log("init done");
}

function restartGoIfExited() {
  if (go.exited) {
    init(options);
    port.postMessage({type: "status", text: "Restarting extension"});
    return true;
  }
  return false;
}

// entry points
chrome.storage.local.get(['options'], function(result) {
  if(result['options']) options = result['options'];
  else options = default_options;
  init(options);
});

setInterval(restartGoIfExited, 120000);