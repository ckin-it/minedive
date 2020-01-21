window.browser = window.browser || window.chrome || window.msBrowser;
let bkg = browser.extension.getBackgroundPage();
let log = bkg.console.log;
let q = '';
let r = [];

let port = browser.runtime.connect();

browser.storage.onChanged.addListener(function(changes, namespace) {
  for (let key in changes) {
    if(key == 'q_'+q) {
      let storageChange = changes[key];
      r = storageChange.newValue;
      show_results();
    }
  }
});

document.getElementById('search').onkeypress = function(e) {
  if(e.key == 'Enter') {
    let s = document.getElementById('search');
    q = s.value;
    search_all();
  }
}

function show_results() {
  let res = document.querySelector('div#results');
  var ol = document.createElement('ol');
  ol.textContent = 'searching for '+q;
  var oldol = res.querySelector('ol');
  if(!r) return;
  r.forEach( function(e) {
    var li = document.createElement('li'); 
    var a = document.createElement('a');
    a.href = e;
    a.textContent = e;
    li.appendChild(a);
    ol.appendChild(li);
  });
  if(oldol) {
    res.insertBefore(ol, oldol);
    log("oldol is present");
  } else {
    res.appendChild(ol);
  }
}

function search_all() {
  search_from_cache();
  port.postMessage({type: 'search', q: q});
}

function search_from_cache() {
  let out = '';
  let tableArray;
  browser.storage.local.get(['q_'+q], function(result) 
  {
    if(result['q_'+q]) tableArray = result['q_'+q];
    r = tableArray;
    show_results();
  });
}

document.addEventListener('DOMContentLoaded', (event) => {
  let url = new URL(document.location);
  q = url.searchParams.get("q");
  if(q) search_from_cache(q);
});
