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

function ws_ping() {
  var d = new Date();
  var n = d.getTime();
  let msg = {
    type: "ping",
    data: n
  };
  connection.send(JSON.stringify(msg));
}

//ws server messages
function ws_connect() {
    connection = new WebSocket(options.ws_server, "json");
  
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
          log('answer');
          log(msg);
          let p = get_peer_by_name(msg.name);
          log(p);
          log('answer from '+p.name);
          accept_answer(p.pc, msg.sdp);
          break;
        }
        case "message": {
          console.log('from '+msg.name+': '+msg.text);
          break;
        }
        case "pong": {
          console.log("pong");
          break;
        }
        case "rejectusername": {
          console.log("(other name in use) username: "+msg.name);
          break;
        }
        case "userlist": {
          log(msg);
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
  