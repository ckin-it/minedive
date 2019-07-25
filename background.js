//let log = console.log;
let log = function() {return;}
let ws_server = 'www.pubbo.it:6504';
//{urls: "stun:stun.l.google.com:19302"},
//const ice_config = {iceServers: [{urls: "stun:pubbo.it:19303"}]};
const ice_config = {iceServers: [{urls: "stun:stun.l.google.com:19302"}]};

let ba = chrome.browserAction;
let status_log = '';

function baSetAllRead() {
  ba.setBadgeBackgroundColor({color: [0, 255, 0, 128]});
  ba.setBadgeText({text: ''});   // <-- set text to '' to remove the badge
}

function baSetGreen(unreadItemCount) {
  ba.setBadgeBackgroundColor({color: [0, 0xdd, 0, 128]});
  ba.setBadgeText({text: '' + unreadItemCount});
}

function baSetRed(unreadItemCount) {
  ba.setBadgeBackgroundColor({color: [0xee, 0, 0, 128]});
  ba.setBadgeText({text: '' + unreadItemCount});
}

function baSetYellow(unreadItemCount) {
  ba.setBadgeBackgroundColor({color: [0xee, 0x8d, 0, 128]});
  ba.setBadgeText({text: '' + unreadItemCount});
}

function baSetUnread(unreadItemCount) {
  ba.setBadgeBackgroundColor({color: [0xff, 0, 0, 128]});
  ba.setBadgeText({text: '' + unreadItemCount});
}


const keyPair = nacl.box.keyPair();
const publicKey = keyPair.publicKey;
const secretKey = keyPair.secretKey;

function inc_nonce(nonce, dyn) {
  for(let i = 1; i <= dyn; i++)
  {
    if(nonce[nonce.length-i] < 0xff) {
      nonce[nonce.length-i]++;
      return true;
    } else {
      nonce[nonce.length-i] = 0;
    }
  }
  return false;
}

const str_publicKey = nacl.util.encodeBase64(publicKey);

function get_peer_by_alias(alias){
  let name = peer_name_from_alias(alias);
  return get_peer_by_name(name);
}

function peer_name_from_alias(alias) {
  return alias_to_peers[alias];
}

let l1_peers = {};
let l2_peers = {};
let alias_to_peers = {};
let channelID = 0;

let minL1 = 3;
let minL2 = 3;

function get_peers_if_needed(){
  let needed = 0;
  if(Object.keys(l1_peers).length < minL1)  {
    needed++;
    ws_get_peers();
  }
  if(Object.keys(l2_peers).length < minL2) {
    needed++;
    ask_l2_all();
  }
  if(needed) {
    baSetYellow(' ');
  } else {
    baSetGreen(' ');
  }
}
//setInterval(get_peers_if_needed, 10000)
setInterval(get_peers_if_needed, 10000);

function lazy_ping() {
  for(let i in l1_peers) {
    let p = l1_peers[i];
    if(p.dc.readyState == 'open') {
      let data = Date.now();
      p.sendJSON({from: myUsername, type: 'ping', text: data});
      p.ping_timeout = setTimeout(function(){
        log('ping timeout');
        delete_l1_peer(p);
      }, 10000);
    }
  }
}
setInterval(lazy_ping, 10000);

function parseL2msg(l2, emsg) {
  let encrypted = nacl.util.decodeBase64(emsg);
  let decrypted = nacl.box.open.after(encrypted, l2.ononce, l2.k)
  let data = nacl.util.encodeUTF8(decrypted);
  inc_nonce(l2.ononce, nacl.box.nonceLength-1);
  return JSON.parse(data);
}

function sendL2JSON(l2, json) {
  if(!l2.pk) {
    //XXX maybe prune
    return;
  }
  let gw = l2.gw;
  let message = nacl.util.decodeUTF8(JSON.stringify(json));
  let encrypted = nacl.box.after(message, l2.inonce, l2.k);
  let emsg = nacl.util.encodeBase64(encrypted);
  let msg = {
    from: myUsername,
    type: 'fwd',
    to: l2.name,
    msg: emsg
  }
  gw.sendJSON(msg);
  inc_nonce(l2.inonce, nacl.box.nonceLength-1);
}

function bcast_l2(opaque) {
  let o = nacl.util.encodeBase64(nacl.util.decodeUTF8(opaque));
  for(k in l2_peers) {
    if(l2_peers.hasOwnProperty(k)) {
      let l = l2_peers[k];
      let gw = get_peer_by_name(l.gw);
      let msg = {
        from: myUsername,
        type: 'fwd',
        to: k,
        msg: o
      }
      gw.sendJSON(msg);
    }
  }
}

function get_l2_key(alias, gw) {
  let msg = {
    type: 'getkey',
    gw: gw,
    alias: alias
  }
  connection.send(JSON.stringify(msg));  
}

function get_l2_apart(tgt) {
  let o = [];
  //log('get_l2_apart: '+tgt);
  for(k in l1_peers) {
    if(l1_peers.hasOwnProperty(k)) {
      let l = l1_peers[k];
      if(l.pc.connectionState == 'connected') {
        if(l.name != myUsername && l.name != tgt && l.alias) {
          o.push(l.alias);
        }
      }
    }
  }
  return o;
}

function send_l2(p) {
  let l2list = get_l2_apart(p.name);
  if(l2list.length) {
    let msg = {
      from: myUsername,
      type: 'l2',
      l2: [p.alias]    
    };
    for(l2 in l2list) {
      log(l2);
      let l = get_peer_by_name(l2);
      if(l) {
        log(l);
        l.sendJSON(msg);
      }
    }
    msg.l2 = l2list;
    log(msg);
    p.sendJSON(msg);
  }
}

function ask_l2(tgt) {
  let p = get_peer_by_name(tgt);
  //log('ask_l2: '+tgt);
  let msg = {
    from: myUsername,
    type: 'getl2'
  }
  p.sendJSON(msg);
}

function ask_l2_all() {
  for(k in l1_peers) {
    if(l1_peers.hasOwnProperty(k)) {
      let l = l1_peers[k];
      if(l.dc.readyState == 'open') ask_l2(l.name);
    }
  }
  return;
}

function resp_google_search_l2(l2, q, l) {
  let x = new XMLHttpRequest();
  let url = 'http://www.google.com/search?q='+q+'&lr='+l+'&hl='+l;
  x.onload = function () {
    if(x.status == 200) {
      log(x);
      let r = extract_results(x.responseText);
      log(r);
      let msg = {
        type: 'resp',
        q: q,
        text: r
      };
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

function delete_l1_peer(o) {
  delete l1_peers[o.name];
  //XXX delete all l2 with o as gw
  o.dc.close();
  o.pc.close();
  log(o.name+' L1 peer lost'); //XXX send to status
  status_log += o.name+' disconnected.<br/>';
  let l2lost = 0;
  for(k in l2_peers) {
    let l = l2_peers[k];
    if(l.gw.name == o.name) {
      delete l2_peers[k];
      l2lost++;
    }
  }
  if(l2lost) status_log += l2lost+' L2 peers lost.<br/>';
}

function create_peer(name, alias) {
  let o = {};
  o.name = name;
  o.alias = alias;
  o.pc = new RTCPeerConnection(ice_config);
  o.dc = o.pc.createDataChannel(name, {negotiated: true, id: 0});
  channelID++;
  o.dc.onopen = () => {
    if(o.dc.readyState != 'open') { 
      log(o.name+' not open yet ('+o.dc.readyState+')');
      return;
    }
    clearTimeout(o.timeout);
    o.sendJSON = (e) => o.dc.send(JSON.stringify(e));
    let ping_text = Date.now();
    o.sendJSON({from: myUsername, type:'ping', text:ping_text});
  }
  o.dc.onmessage = m => handle_l1_msg(m);
  o.dc.onclose = () => log(o.name+' closed');
  o.pc.oniceconnectionstatechange = e => {
    //log('ICE '+o.name +' is '+o.pc.iceConnectionState);
  }
  o.pc.onconnectionstatechange = e => {
    log(o.name +' is '+o.pc.connectionState);
    if(o.pc.connectionState == 'failed' || o.pc.connectionState == 'disconnected') {
      delete_l1_peer(o);
    }
  }
  o.pc.oniceconnectionstatechange = e => {
    log(o.name +' ICE is '+o.pc.iceConnectionState);
  }
  o.pc.onicecandidate = ice => {
    //log(ice.candidate);
  }
  log('peer '+o.name+' created');
  o.timeout = setTimeout(function(){
    log('connection timeout'); 
    delete_l1_peer(o);
  }, 300000);
  return o;
}

function handle_resp_l2(p, msg) {
  log('resp for '+msg.q+' from '+p.name);
  let r = msg.text;
  chrome.storage.local.get(['q_'+msg.q], function(result) {
    log('here storage.local.get');
    let a = result[msg.q];
    let c = 0;
    if(a) {
      log(r.length);
      log(r.lenght);
      for(let i = 0; i < r.length; i++) {
        if(!a.includes(r[i])) {
          a.push(r[i]);
          c = 1;
        } else {
          log(r[i]+' yet present ');
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

function handle_l2_msg(msg, gw) {
  let l2 = l2_peers[msg.ori];
  if(!l2) {
    log(msg.ori + 'not found');
    l2_peers[msg.ori] = {
      name: msg.ori, 
      pk:'', 
      gw: gw, 
      state: 'from-msg', 
      msg_ignored: 1
    };
    get_all_keys();
    return;
  }
  if(!l2.k) {
    l2_peers[l2].msg_ignored++;
    get_all_keys();
  }
  let m = parseL2msg(l2, msg.msg);
  log('XXX here i show the unpacked thing');
  log(m);

  switch(m.type) {
    case 'search':
      log(m);
      resp_google_search_l2(l2, m.q);
      break;
    case 'resp':
      handle_resp_l2(l2, m);
      break;
  }
}

function send_peer_unreachable(peer, unreach) {
  let msg = {
    type: 'unreachable',
    l2: unreach
  };
  peer.sendJSON(msg);
}

function handle_l1_msg(e) {
  let from = e.currentTarget.label;
  let msg = JSON.parse(e.data);
  if(msg.from == from) var p = get_peer_by_name(from);
  else {log('peer '+from+'trying to spoof '+msg.from+' msg ignored');}
  if(!p) return null;
  //log(msg);
  switch(msg.type) {
    case 'unreachable': {
      log(msg.l2+' unreachable');
      delete l2_peers[msg.l2];
      break;
    }
    case 'fwd': {
      if(msg.to == myUsername) {handle_l2_msg(msg, p); return;}
      log('XXX here i fwd this');
      log(msg);
      let dest_name = alias_to_peers[msg.to];
      if(!dest_name) {
        log('unknown alias '+msg.to); 
        send_peer_unreachable(p, msg.to); 
        return;
      }
      let dest_peer = get_peer_by_name(dest_name);
      if(!dest_peer) {
        log('peer unreachable '+dest_name); 
        send_peer_unreachable(p, msg.to); 
        return;
      }
      msg.from = myUsername;
      msg.ori = p.alias;
      msg.to = dest_name;
      dest_peer.sendJSON(msg);
      break;
    }
    case 'search': {
      log(msg);
      resp_google_search(p, msg.q);
      break;
    }
    case "l2": {
      log(msg);
      for(l in msg.l2) {
        l2_peers[msg.l2[l]] = {name: msg.l2[l], pk:'', gw:p, state: 'new', msg_ignored: 0};
      }
      get_all_keys();
      break;
    }
    case "getl2": {
      //log(msg);
      send_l2(p);
      break;
    }
    case 'resp': {
      handle_resp(p, msg);
      break;
    }
    case 'ping': {
      let ping_text = msg.text;
      let rep = {
        from: myUsername,
        type: 'pong',
        text: ping_text,
      }
      p.sendJSON(rep);
      break;
    }
    case 'pong': {
      let jiffies = Date.now();
      let ping = msg.text;
      let latency = jiffies-ping;
      //log('roundtrip '+p.name+': '+latency);
      if(p.ping_timeout) clearTimeout(p.ping_timeout);
      break;
    }
  }
}
function makeid(length) {
   let result = '';
   let caps = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
   let characters = '_abcdefghijklmnopqrstuvwxyz0123456789';
   let charactersLength = characters.length;
   result = caps.charAt(Math.floor(Math.random() * caps.length));
   for (let i = 0; i < length-1; i++ ) {
      result += characters.charAt(Math.floor(Math.random() * characters.length));
   }
   return result;
}

let myUsername = null;
let connection = null;

function get_all_keys() {
  for(l in l2_peers) {
    let p = l2_peers[l];
    if(!p.pk) ws_get_key(p.name, p.gw);
  }
}

function ws_get_alias(peer_name) {
  let msg = {
    name: myUsername,
    date: Date.now(),
    id: clientID,
    type: "getalias",
    l1: peer_name
  };
  //log(msg);
  connection.send(JSON.stringify(msg));
}

function ws_get_key(alias, gw) {
  let msg = {
    name: myUsername,
    pk: str_publicKey,
    date: Date.now(),
    id: clientID,
    alias: alias,
    gw: gw.name,
    type: "getkey"
  };
  //log(msg);
  connection.send(JSON.stringify(msg));
}

function ws_get_peers() {
  let msg = {
    name: myUsername,
//    pk: str_publicKey,
    date: Date.now(),
    id: clientID,
    type: "getpeers"
  };
  //log(msg);
  connection.send(JSON.stringify(msg));
}

function set_username() {
  let msg = {
    name: makeid(10),
    pk: str_publicKey,
    date: Date.now(),
    id: clientID,
    type: "username"
  };
  myUsername = msg.name;
  log('my username is: '+myUsername);
  connection.send(JSON.stringify(msg));
}

function ws_send_sdp(_tgt, _type, _sdp) {
  let msg = {
    name: myUsername,
    date: Date.now(),
    target: _tgt,
    type: _type,
    sdp: _sdp
  };
  connection.send(JSON.stringify(msg));
}

function get_peer_by_name(name) {
  return l1_peers[name];
}

async function create_all_offers() {
  for(k in l1_peers) {
    create_offer(l1_peers[k].pc, l1_peers[k].name);
  };
}

async function create_offer(pc, user) {
  log('initiating '+user);
  await pc.setLocalDescription(await pc.createOffer());
  pc.onicecandidate = ({candidate}) => {
    if (candidate) return;
    ws_send_sdp(user, 'offer', pc.localDescription.sdp);
  };
}

async function accept_offer(pc, user, offer_value) {
  log('accept offer '+user);
  if (pc.signalingState != "stable") return;
  await pc.setRemoteDescription({type: "offer", sdp: offer_value});
  await pc.setLocalDescription(await pc.createAnswer());
  pc.onicecandidate = ({candidate}) => {
    if (candidate) return;
    ws_send_sdp(user, 'answer', pc.localDescription.sdp);
  };
};

function accept_answer(pc, answer_value) {
  if (pc.signalingState != "have-local-offer") return;
  pc.setRemoteDescription({type: "answer", sdp: answer_value});
};

//ws server messages
function ws_connect() {
  connection = new WebSocket("wss://" + ws_server, "json");

  connection.onopen = function(evt) {
    baSetYellow(' ');
  };

  connection.onclose = function(evt) {
    baSetRed(' ');
    log('WS server disconnected');
    setTimeout(ws_connect, 10000);
  };

  connection.onmessage = function(evt) {
    let text = "";
    let msg = JSON.parse(evt.data);
    let time = new Date(msg.date);
    let timeStr = time.toLocaleTimeString();

    switch(msg.type) {
      case "id": {
        clientID = msg.id;
        set_username();
        break;
      }
      case "username": {
        //console.log('username: '+msg.name);
        get_peers_if_needed();
        break;
      }
      case "key": {
        let l2 = l2_peers[msg.alias];
        if(l2) {
          if(!msg.key) {
            log(msg.alias+' unknown from server');
            delete l2_peers[msg.alias];
            return;
          }
          l2.pk = nacl.util.decodeBase64(msg.key);
          l2.k = nacl.box.before(l2.pk, secretKey);
          l2.inonce = new Uint8Array(nacl.box.nonceLength);
          l2.ononce = new Uint8Array(nacl.box.nonceLength);
          if(l2.state == 'from-msg') {
            while(l2.msg_ignored > 0) {
              log('still l2.msg_ignored '+l2.msg_ignored);
              inc_nonce(l2.ononce, nacl.box.nonceLength-1);
              l2.msg_ignored--;
            }
          }
          l2.state = 'ok';
          status_log += 'L2 available<br/>';
          baSetGreen(' ');
          log(l2_peers[msg.alias]);
        }
        break;
      }
      case "alias": {
        let p = get_peer_by_name(msg.l1);
        if(p) {
          let old_alias = p.alias;
          if(old_alias) alias_to_peers.remove([old_alias]);
          p.alias = msg.alias;
          log('alias create for '+p.name);
        }
        alias_to_peers[msg.alias] = p.name;
        break;
      }
      case "offer": {
        log('offer from '+msg.name);
        let p = get_peer_by_name(msg.name);
        if(!p) {
          if(!msg.alias) log('missing alias to peer '+msg.name);
          let p = create_peer(msg.name, '');
          p.from_signaling = false;
          l1_peers[p.name] = p;
          ws_get_alias(p.name);
        }
        accept_offer(p.pc, p.name, msg.sdp);
        break;
      }
      case "answer": {
        let p = get_peer_by_name(msg.name);
        log('answer from '+p.name);
        accept_answer(p.pc, msg.sdp);
        break;
      }
      case "message": {
        console.log('from '+msg.name+': '+msg.text);
        break;
      }
      case "rejectusername": {
        console.log("(other name in use) username: "+msg.name);
        break;
      }
      case "userlist": {
        let i;
        for (i=0; i < msg.users.length; i++) {
          if(msg.users[i].name != myUsername) {
            if(!l1_peers[msg.users[i].name]) {
              let p = create_peer(msg.users[i].name, msg.users[i].alias);
              p.from_signaling = msg.contact;
              p.toprune = false;
              p.alias = msg.users[i].alias;
              l1_peers[p.name] = p;
              log(p.alias);
              alias_to_peers[p.alias] = p.name;
              if(msg.contact) create_offer(p.pc, p.name);
            }
          }
        }
        break;
      }
    }

    if (text.length) {
      console.log(text);
    }
  };
}

chrome.storage.onChanged.addListener(function(changes, namespace) {
  for (let key in changes) {
    let storageChange = changes[key];
    log('Storage key "%s" in namespace "%s" changed. ' +
                      'Old value was "%s", new value is "%s".',
                      key,
                      namespace,
                      storageChange.oldValue,
                      storageChange.newValue);
    }
});

function emptyDB() {
  chrome.storage.local.clear(function(){bkg.console.log('data clear');});
}

function dumpDB() {
  chrome.storage.local.get(null, function(items) {
    var allKeys = Object.keys(items);
    console.log(allKeys);
  });
}

let r = {};
let bookmarks = {state: 0, a: []};
let history = {state: 0, a: []};
let idle = false;


function report_peers() {
  log("aliases");
  log(alias_to_peers);
  log("L1");
  log(l1_peers);
  log("L2");
  log(l2_peers);
}

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

chrome.runtime.onConnect.addListener(function(port) {
  port.onMessage.addListener( function(msg) {
    switch(msg.type) {
      case 'status':
        //XXX sending just peers number, enabling search or not
        let peers_number = '';
        peers_number  = 'L1 peers: '+Object.keys(l1_peers).length+'</br>';
        peers_number += 'L2 peers: '+Object.keys(l2_peers).length+'</br>';
        port.postMessage({type: "status", data: peers_number});
        break;
      case 'search':
        //XXX currently sending search to all L2 peers
        for(k in l2_peers) {
          let p = l2_peers[k];
          if(p.gw.dc.readyState == 'open')
          {
            let lang = navigator.language || navigator.userLanguage;
            let m = {
              type: 'search',
              l: lang,
              q: msg.q
            };
            sendL2JSON(p, m);
          }
          //XXX sending search_status / err
          //port.postMessage({type: "search_status", data: XXX});
        };
      break;
    }
  });
});

ws_connect();
