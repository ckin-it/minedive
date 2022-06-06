window.browser = window.chrome || window.browser || window.msBrowser;
let bkg = browser.extension.getBackgroundPage();
let log = bkg.console.log;
let q = '';
document.r = [];
search_pb = document.getElementById('search-pb');
search_pb.onclick = function(element) {
  var newURL = browser.runtime.getURL('/ui/search.html');
  log(newURL);
  browser.tabs.create({ url: newURL});
}

options_pb = document.getElementById('options-pb');
options_pb.onclick = function(element) {
  var newURL = browser.runtime.getURL('/ui/options.html');
  log(newURL);
  browser.tabs.create({ url: newURL});
}

var port = browser.runtime.connect();

port.onMessage.addListener(function(msg) {
  switch(msg.type) {
    case 'status':
      log('status')
      log(msg)
      update_popup('status-p', msg.text, msg.l1, msg.l2);
      break;
  }
});

window.onload = function() {
  port.postMessage({type: 'status'});
}

browser.storage.onChanged.addListener(function(changes, namespace) {
  for (var key in changes) {
    if(key == q) {
      var storageChange = changes[key];
      document.r = storageChange.newValue;
      show_results();
    }
  }
});

function update_popup(s, _text, _l1, _l2)
{
  var res = document.querySelector('div#'+s);
  let status  = '';
  if(_text) status += sanitizeHTML(_text);
  if(_l1) status += '\r\nL1 peers: '+sanitizeHTML(_l1);
  if(_l2) {
    status += '\r\nL2 peers: '+sanitizeHTML(_l2);
    if(_l2 > 0) search_pb.disabled = false;
    else search_pb.disabled = true;
  } 
  else search_pb.disabled = true;
  if(status) res.textContent = status;
}
