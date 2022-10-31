let avoid_domains = 
['webcache.googleusercontent.com',
 'translate.google.com'
 ];
let accept_protocols = ['http:', 'https:']

function extract_results(_t)
{
  let out = [];
  const parser = new DOMParser()
  const parsed = parser.parseFromString(_t, `text/html`)
  let as = parsed.querySelector("div#res").querySelectorAll('a');
  for(let i = 0; i < as.length; i++) {
    let a = as[i];
    let url;
    if (a.href != "") {
    try { url = new URL(a.href); 
      if(accept_protocols.includes(url.protocol) && !avoid_domains.includes(url.host)) {
        if(!out.includes(a.href)) out.push(a.href);
      }
    } catch(error){log("new URL error:", a.href, error);}
    }
  }
  return out;
}

function resp_google_search_l2(l2, q, l, key, nonce) {
    let x = new XMLHttpRequest();
    sq = encodeURIComponent(q);
    let url = 'http://www.google.com/search?q='+sq+'&lr='+l+'&hl='+l;
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
        log(r);
        //sendL2JSON(l2, msg);
        log("L2 in js:", l2);
        if(l2){replyL2(l2, key, nonce, msg);}
      } else {
        log(x.status);
      }
    };
    x.onerror = function(err) {
      console.log('XMLHTTPRequest error');
      console.log(err)
    };
    x.open('GET', url);
    x.send()
  }
