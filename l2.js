function get_peer_by_alias(alias){
  let name = peer_name_from_alias(alias);
  return get_peer_by_name(name);
}
  
function peer_name_from_alias(alias) {
  return alias_to_peers[alias];
}
  
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
    if(l2list.length == 0 ) return;
    let msg = {
      from: myUsername,
      type: 'l2',
      l2: [p.alias],
      i: 0
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
    msg.i = 1;
    log(msg);
    p.sendJSON(msg);
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
        if(l.dc) {
          if(l.dc.readyState == 'open') ask_l2(l.name);
        } else {
          log(l.name + "has no dc, has pc?" + l.pc);
        }
      }
    }
    return;
  }

  function handle_resp_l2(p, msg) {
    log('resp for '+msg.q+' from '+p.name);
    status_log = 'connected\r\n';
    let r = msg.text;
    chrome.storage.local.get(['q_'+msg.q], function(result) {
      log('here storage.local.get');
      let a = result['q_'+msg.q];
      let c = 0;
      if(a) {
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
        let q = encodeURIComponent(m.q);
        let l = encodeURIComponent(m.l);
        resp_google_search_l2(l2, q, l);
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
  
