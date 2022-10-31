window.browser = window.browser || window.chrome || window.msBrowser;
let q = '';
let r = [];

let port = window.browser.runtime.connect();

//port.postMessage({type: 'regpage'});

port.onMessage.addListener(function(msg, p){
  const obj = JSON.parse(msg);
  log("LISTENER TRIGGERED");
  log(msg);
  switch(obj.type) {
    case "resp":
      show_results(obj.q, obj.text);
      break;
    case "respv2":
      show_results(obj.q, obj.text);
      break;
    }
});

// browser.storage.onChanged.addListener(function(changes, namespace) {
//   for (let key in changes) {
//     if(key == 'q_'+q) {
//       let storageChange = changes[key];
//       r = storageChange.newValue;
//       show_results();
//     }
//   }
// });

document.getElementById('search-button').onclick = function(e) {
  let s = document.getElementById('search');
  q = encodeURIComponent(s.value);
  //XXX other side decodeURIComponent
  search_all(s.value);
}

document.getElementById('search').onkeydown = function(e) {
  if(e.key == 'Enter') {
    let s = document.getElementById('search');
    q = encodeURIComponent(s.value);
    //XXX other side decodeURIComponent
    search_all(s.value);
    return false;
  }
}

function is_accepted_result(str) {
  return true; //XXX
  var proto = str.substr(0, str.indexOf(':'));
  switch (proto) {
    case "http":
    case "https":
      return true;
    default:
      return false;
  }
}

function show_results(q, r) {
  //log("show results triggered", q, r);
  //store in memory in store[q][url]
  //log('show results in results-'+btoa(q)+' from '+q);
  let res = document.getElementById('results-'+btoa(q));
  if (res != null) {
    res.textContent = "Results for "+q;
  } else {
    log('results-'+btoa(q)+' is null (because DOM direct load?)');
    create_results_box(q);
  }
  var ol = document.createElement('ol');
  res.appendChild(ol); //XXX
  var oldol = res.querySelector('ol');
  if(!r) return;
  log(r);
  r.forEach( function(e) {
    //log("adding", e);
    var li = document.createElement('li'); 
    var a = document.createElement('a');
    if(is_accepted_result(e)) {
      a.href = e;
      a.textContent = e;
      li.appendChild(a);
      ol.appendChild(li);
    }
  });
  if(oldol) {
    res.insertBefore(ol, oldol);
    //log("oldol is present");
  } else {
    res.appendChild(ol);
  }
}

function create_results_box(inval) {
  let val = btoa(inval);
  //log("creating results-"+val+' from '+inval);
  let div = document.getElementById('results-'+val);
  if (div == null) {
    div = document.createElement('div');
    div.setAttribute('id', 'results-'+val);
    div.classList.add('results');
    div.innerHTML = 'searching for '+inval+'...';
    let w = document.getElementById('searchform');
    w.after(div);
  }
}

function search_all(val) {
  //search_from_cache(val);
  create_results_box(val);
  port.postMessage({type: 'search', q: val});
}

// function search_from_cache(val) {
//   let out = '';
//   let tableArray;
//   let bval = btoa(val);
//   browser.storage.local.get(['q_'+bval], function(result) 
//   {
//     if(result['q_'+bval]) tableArray = result['q_'+bval];
//     r = tableArray;
//     show_results();
//   });
// }

// document.addEventListener('DOMContentLoaded', (event) => {
//   let url = new URL(document.location);
//   log('DOMContentLoaded triggered');
//   q = url.searchParams.get("q");
//   log('DOMContentLoaded Q['+q+']');
//   if(q) search_from_cache(q);
// });
