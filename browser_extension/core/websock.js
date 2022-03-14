//const { SSL_OP_EPHEMERAL_RSA } = require("constants");

function get_all_keys() {
  for(l in l2_peers) {
    let p = l2_peers[l];
    //log('XXX get key for', p.name);
    if(!p.pk) ws_get_key(p.name, p.gw);
  }
}

function ws_get_alias(peer_name) {
  let msg = {
    name: TID,
    date: Date.now(),
    id: clientID,
    type: "getalias",
    d0: peer_name
  };
  //log(msg);
  connection.send(JSON.stringify(msg));
}
  
function ws_get_key(alias, gw) {
  let msg = {
    type: "getkey",
    d0: alias,
    d1: gw.name,
    //d2: nacl.util.encodeBase64(kp.publicKey), - sent in set_username / gettid
    //pk: nacl.util.encodeBase64(kp.publicKey),
    //id: clientID,
    //date: Date.now()
  };
  //log(msg);
  connection.send(JSON.stringify(msg));
}

function ws_get_peers() {
  let msg = {
  //    pk: str_publicKey,
  //    date: Date.now(),
  //    id: clientID,
    type: "getpeers",
  //  d0: TID,
  };
  //log(msg);
  connection.send(JSON.stringify(msg));
}
  
function set_username() {
  let msg = {
    type: "gettid",
    d0: nacl.util.encodeBase64(kp.publicKey)
  };
  connection.send(JSON.stringify(msg));
  //log("gettid sent");
}
  
function resume_tid() {}

function ws_send_sdp(_tgt, _type, _sdp) {
    let msg = {
      d0: TID,
      date: Date.now(),
      d1: _tgt,
      type: _type,
      d2: _sdp
    };
    connection.send(JSON.stringify(msg));
  }

function ws_ping() {
  let d = new Date();
  let n = d.getTime().toString();
  let msg = {
    type: "ping",
    d0: n
  };
  connection.send(JSON.stringify(msg));
}

//ws server messages
function ws_connect() {
    connection = new WebSocket(options.ws_server, "json");
  
    connection.onopen = function(evt) {
      baSetYellow(' ');
      set_username();
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
      if (msg.type != "pong") {
        //log(msg.type)
      }
  
      switch(msg.type) {
        case "id": {
          clientID = msg.id;
          set_username();
          break;
        }
        case "tid": {
          TID = msg.d0;
          TKID = msg.d1;
          log("my username is:", TID, "(", TKID,")");
        }
        case "username": {
          //console.log('username: '+msg.name);
          //get_peers_if_needed();
          break;
        }
        case "key": {
          let l2 = l2_peers[msg.d0];
          if(l2) {
            if(!msg.d1) {
              log(msg.d0+' unknown from server');
              delete l2_peers[msg.d0];
              return;
            }
            l2.pk = nacl.util.decodeBase64(msg.d1);
            l2.k = nacl.box.before(l2.pk, kp.secretKey);
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
            status_log = 'L2 connected\r\n';
            baSetGreen(' ');
            //log(l2_peers[msg.d0]);
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
          //log('offer from '+msg.d0);
          let p = get_peer_by_name(msg.d0);
          if(!p) {
            log("how do I get here???");
            //if(!msg.alias) log('missing alias to peer '+msg.d0);
            //let p = create_peer(msg.d0, '');
            //add_dc_handler(p);
            //p.from_signaling = false;
            //l1_peers[p.name] = p;
            //ws_get_alias(p.name);
            //status_log += 'L1 offer from '+msg.name+'\r\n';
          }
          if(p.pc) accept_offer(p.pc, p.name, msg.d2);
          else {
            log("XXX"+p.name +" has no " +p.pc);
          }
          break;
        }
        case "answer": {
          //log('ANSWER XXX answer SDP:');
          //log(msg.d2);
          //log(msg);
          let p = get_peer_by_name(msg.d0);
          //log(p);
          //log('answer from '+p.name);
          status_log += 'L1 answer received from '+msg.d0+'\r\n';
          accept_answer(p.pc, msg.d2);
          break;
        }
        case "message": {
          log('from '+msg.name+': '+msg.text);
          break;
        }
        case "pong": {
          //let n0 = msg.d0;
          //let d = new Date();
          //log("RTT", d.getTime() - parseFloat(n0));
          break;
        }
        case "rejectusername": {
          log("(other name in use) username: "+msg.name);
          break;
        }
        case "userlist": {
          //log(msg);
          let u = nacl.util.decodeBase64(msg.d1)
          if (u != "") {
          let user = JSON.parse(String.fromCharCode.apply(null, u));
          let i;
          if(user.d0 != TID) {
            if(!l1_peers[user.d0]) {
              let p = create_peer(user.d0, user.d1);
              p.from_signaling = msg.d0;
              p.toprune = false;
              p.alias = user.d1;
              //log("ALIAS", user.d0, "alias", user.d1);
              l1_peers[p.name] = p;
              alias_to_peers[p.alias] = p.name;
              if(msg.d0 == 1) {
                create_dc(p);
                create_offer(p.pc, p.name);
                //log("dc created for", p.name);
              } else {
                //log("dc handler created for", p.name);
                add_dc_handler(p);
              }
            } else {
              //XXX handle not connected case
            }
          }
          }
          break;
        }
      }
  
      if (text.length) {
        log(text);
      }
    };
  }
  