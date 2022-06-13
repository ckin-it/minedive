let connectedPorts = [];

chrome.runtime

chrome.runtime.onConnect.addListener(function(port) {
  connectedPorts.push(port);
  log("connecting new port [", connectedPorts.length, "]");
  port.onMessage.addListener( function(msg) {
    log("port print:", msg);
    switch(msg.type) {
      case 'status':
        let peers_number_l1 = MinediveGetNL1(); //XXX fix this
        let peers_number_l2 = MinediveGetNL2();
        let status_log = "it's all good man";
        //let peers_number_l1 = Object.keys(l1_peers).length;
        //let peers_number_l2 = Object.keys(l2_peers).length;
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

// entry points
chrome.storage.local.get(['options'], function(result) {
  if(result['options']) options = result['options'];
  else options = default_options;
  init(options);
});
