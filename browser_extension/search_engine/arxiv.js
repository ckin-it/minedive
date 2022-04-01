let arxiv_avoid_domains = 
['webcache.googleusercontent.com',
 'translate.google.com'
 ];
let arxiv_accept_protocols = ['http:', 'https:']
let glob_t;
function arxiv_extract_results(_t)
{
glob_t = _t;
  let out = [];
  let as = [];
  let c = document.implementation.createHTMLDocument().documentElement;
  c.innerHTML = _t;
  //log(_t);
  try {
  as = c.querySelector("li.arxiv-result").querySelectorAll('a');
  } catch(error) {}
  for(let i = 0; i < as.length; i++)
  {
    let a = as[i];
    let url
    try { url = new URL(a.href); 
      if(arxiv_accept_protocols.includes(url.protocol) && !arxiv_avoid_domains.includes(url.host)) {
        if(!out.includes(a.href)) out.push(a.href);
      }
    } catch(error){log(a.href, error);}
  }
  return out;
}

function arxiv_resp_search_l2(l2, q, l) {
    let x = new XMLHttpRequest();
    let url = "https://arxiv.org/search/?query="+q+"&searchtype=all&abstracts=hide&order=-announced_date_first&size=100";
    log(url)
    x.onload = function () {
      if(x.status == 200) {
        let r = arxiv_extract_results(x.responseText);
        let msg = {
          type: 'resp',
          q: q,
          text: r
        };
        log("sending...")
        //sendL2JSON(l2, msg);
        replyL2(l2, msg);
      }
    };
    x.onerror = function(err) {
      console.log('XMLHTTPRequest error');
      console.log(err)
    };
    x.open('GET', url);
    x.send()
  }
