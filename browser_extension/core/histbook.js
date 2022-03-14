let r = {}; //XXX what's that?

let bookmarks = {state: 0, a: []};
let history = {state: 0, a: []};
let idle = false;

//XXX handler to add new bookmark and history to arrays when added to system
function proc_bookmarks() {
    switch(bookmarks.state) {
    case 0:
      chrome.bookmarks.getTree(function (bookmarkTree) {
        for(n in bookmarkTree) {
          bookmarks.a.push({t:n.title, u:n.url});
        }
      });
      bookmarks.state = 1;
      break;
    case 1:
      for(let i = 0; i < bookmarks.a.length; i++) {
        let a = bookmarks.a.pop();
        log('process '+a.t+' '+a.u);
        if(!idle) return;      
      }
      break;
    }
  }
  
  function proc_hist() {
    switch(history.state) {
    case 0:
      chrome.history.search({'text': ''}, function(h) {
        for (let i = 0; i < h.length; ++i) {
          history.a.push({t:h[i].title, u: h[i].url});
        }
        history.state = 1;
      });
      break;
    case 1:
      for(let i = 0; i < history.a.length; i++) {
        //XXX process 1 entry and check state
        let a = history.a.pop();
        log('process '+a.t+' '+a.u);
        if(!idle) return;
      }
      break;
    }
  }
  
  chrome.idle.onStateChanged.addListener(function(newState) {
    //XXX
    if(newState == 'XXXidle') {
      proc_hist();
      proc_bookmarks();
      idle = true;
    } else {
      idle = false;
    }
  });
  