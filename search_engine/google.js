function search_google(q) {
    let x = new XMLHttpRequest();
    let url = 'http://www.google.com/search?q=' +  q;
    x.onload = function () {
      if(x.status == 200) {
        log(x);
        log(extract_results(x.responseText));
      }
    };
    x.onerror = function(err) {
      console.log(err)
    };
    x.open('GET', url);
    x.send()
  }

let avoid_domains = 
['webcache.googleusercontent.com',
 'translate.google.com'
 ];
let accept_protocols = ['http:', 'https:']

function extract_results(t)
{
  let out = [];
  let c = document.implementation.createHTMLDocument().documentElement;
  c.innerHTML = t;
  let as = c.querySelector("div.srg").querySelectorAll('a');
  for(let i = 0; i < as.length; i++)
  {
    let a = as[i];
    let url = new URL(a.href);
    if(accept_protocols.includes(url.protocol) && 
       !avoid_domains.includes(url.host)) {
      if(!out.includes(a.href)) out.push(a.href);
    }
  }
  return out;
}

function handle_resp(p, msg) {
  log('resp for '+msg.q+' from '+p.name);
  let r = msg.text;
  chrome.storage.local.get(['q_'+msg.q], function(result) {
    log('here');
    let a = result[msg.q];
    let c = 0;
    if(result[msg.q]) {
      for(let i = 0; i < r.length; i++) {
        log(r[i]);
        if(!a.includes(r[i])) {
          log('new result');
          a.push(r[i]);
          c = 1;
        }
      }
    } else {
      log('new key');
      a = r;
      c = 1;
    }
    if(c) {
      log('update key '+msg.q);
      chrome.storage.local.set({['q_'+msg.q]: a}, function () {log('key updated');});
    }
  });
}

function resp_google_search(p, q, l) {
    let x = new XMLHttpRequest();
    let url = 'http://www.google.com/search?q='+q+'&lr='+l+'&hl='+l;
    x.onload = function () {
      if(x.status == 200) {
        log(x);
        let r = extract_results(x.responseText);
        let msg = {
          from: myUsername,
          type: 'resp',
          q: q,
          text: r
        };
        p.dc.send(JSON.stringify(msg));
      }
    };
    x.onerror = function(err) {
      console.log('XMLHTTPRequest error');
      console.log(err)
    };
    x.open('GET', url);
    x.send()
  }
  