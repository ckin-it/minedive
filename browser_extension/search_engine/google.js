let avoid_domains = 
['webcache.googleusercontent.com',
 'translate.google.com'
 ];
let accept_protocols = ['http:', 'https:']

function extract_results(_t)
{
  let out = [];
  let c = document.implementation.createHTMLDocument().documentElement;
  c.innerHTML = _t;
  log(_t);
  let as = c.querySelector("div#res").querySelectorAll('a');
  for(let i = 0; i < as.length; i++)
  {
    let a = as[i];
    let url
    try { url = new URL(a.href); 
      if(accept_protocols.includes(url.protocol) && !avoid_domains.includes(url.host)) {
        if(!out.includes(a.href)) out.push(a.href);
      }
    } catch(error){log(a.href, error);}
  }
  return out;
}

function resp_google_search_l2(l2, q, l) {
    let x = new XMLHttpRequest();
    let url = 'http://www.google.com/search?q='+q+'&lr='+l+'&hl='+l;
    log(url)
    x.onload = function () {
      if(x.status == 200) {
        let r = extract_results(x.responseText);
        let msg = {
          type: 'resp',
          q: q,
          text: r
        };
        log("sending...")
        sendL2JSON(l2, msg);
      }
    };
    x.onerror = function(err) {
      console.log('XMLHTTPRequest error');
      console.log(err)
    };
    x.open('GET', url);
    x.send()
  }
