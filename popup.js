window.browser = window.chrome || window.browser || window.msBrowser;
let bkg = browser.extension.getBackgroundPage();
let log = bkg.console.log;
let q = '';
document.r = [];

document.getElementById('open').onclick = function(element) {
  var newURL = browser.runtime.getURL('/search.html');
  browser.tabs.create({ url: newURL});
}

var port = browser.runtime.connect();

port.onMessage.addListener(function(msg) {
  switch(msg.type) {
    case 'status':
      update_popup('status', msg.data);
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

function update_popup(s, _text)
{
  var res = document.querySelector('div#'+s);
  res.innerHTML = _text;
}

