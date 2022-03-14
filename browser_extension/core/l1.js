let channelID = 0; //XXX used somewhere?

function lazy_ping() {
  for(let i in l1_peers) {
    let p = l1_peers[i];
    if(p.dc) {
      if(p.dc.readyState == 'open') {
        let data = Date.now();
        p.sendJSON({from: TID, type: 'ping', text: data});
        p.ping_timeout = setTimeout(function(){
          log('ping timeout');
          delete_l1_peer(p);
        }, 10000);
      }
    } else {
      log(p.name+' has no DC yet');
    }
  }
}

function add_dc_handler(p) {
  p.pc.ondatachannel = e => {
    //log("on data channel!");
    p.dc = e.channel;
    p.timeout = setTimeout(function(){
      log(p.name+' connection timeout');
      delete_l1_peer(p);
    }, 300000);

    p.dc.onopen = () => {
      //log("opening data channel")
      if(p.dc.readyState != 'open') {
        log(p.name+' not open yet ('+p.dc.readyState+')');
        return;
      }
      clearTimeout(p.timeout);
      p.sendJSON = (e) => p.dc.send(JSON.stringify(e));
      let ping_text = Date.now();
      p.sendJSON({from: TID, type:'ping', text:ping_text});
    }
    p.dc.onmessage = m => handle_l1_msg(m);
    p.dc.onclose = () => log(p.name+' closed');
  }
}

function create_dc(o) {
  o.dc = o.pc.createDataChannel(TID+o.name); //XXX non nego
  o.dc.onopen = () => {
    //log("opening data channel")
    if(o.dc.readyState != 'open') {
      log(o.name+' not open yet ('+o.dc.readyState+')');
      return;
    }
    clearTimeout(o.timeout);
    o.sendJSON = (e) => o.dc.send(JSON.stringify(e));
    let ping_text = Date.now();
    o.sendJSON({from: TID, type:'ping', text:ping_text});
  }
  o.dc.onmessage = m => handle_l1_msg(m);
  o.dc.onclose = () => log(o.name+' closed');
  log('dc '+o.name+' created');
  o.timeout = setTimeout(function(){
    log(o.name+' connection timeout');
    delete_l1_peer(o);
  }, 300000);
}

function create_peer(name, alias) {
  let o = {};
  o.name = name;
  o.alias = alias;
  o.pc = new RTCPeerConnection({iceServers: options.stun});
  channelID++;
  o.pc.oniceconnectionstatechange = e => {
    //log('ICE cs change', o.name, 'is', o.pc.iceConnectionState);
  }
  o.pc.onconnectionstatechange = e => {
    //log(o.name +' is '+o.pc.connectionState);
    if(o.pc.connectionState == 'failed' || o.pc.connectionState == 'disconnected') {
      delete_l1_peer(o);
    }
  }
  o.pc.onicecandidate = ice => {
    //log('OnICECandidate '+ice.candidate);
  }
  //log('peer '+o.name+' created');
  return o;
}

function create_peer_with_datachannel(name, alias) {
  let o = {};
  o.name = name;
  o.alias = alias;
  o.pc = new RTCPeerConnection({iceServers: options.stun});
  o.dc = o.pc.createDataChannel(name, {negotiated: true, id: 0});
  //o.dc = o.pc.createDataChannel(name, {negotiated: true, protocol: "string", id: 0});
  //o.dc = o.pc.createDataChannel(name, {negotiated: true, id: 0, maxRetransmits: 30});
  //o.dc = o.pc.createDataChannel(name, {maxRetransmits: 3});
  channelID++;
  o.dc.onopen = () => {
    log("opening data channel")
    if(o.dc.readyState != 'open') { 
      log(o.name+' not open yet ('+o.dc.readyState+')');
      return;
    }
    clearTimeout(o.timeout);
    let ping_text = Date.now();
    o.sendJSON({from: TID, type:'ping', text:ping_text});
  }
  o.sendJSON = (e) => o.dc.send(JSON.stringify(e));
  o.dc.onmessage = m => handle_l1_msg(m);
  o.dc.onclose = () => log(o.name+' closed');
  o.pc.oniceconnectionstatechange = e => {
    log('ICE '+o.name +' is '+o.pc.iceConnectionState);
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
    log(ice.candidate);
  }
  log('peer '+o.name+' created');
  o.timeout = setTimeout(function(){
    log(o.name+' connection timeout');
    delete_l1_peer(o);
  }, 300000);
  return o;
}

function get_peer_by_name(name) {
    return l1_peers[name];
  }
  
async function create_all_offers() {
  for(k in l1_peers) {
    create_offer(l1_peers[k].pc, l1_peers[k].name);
  };
}
  
function handle_l1_msg(e) {
  //log(e);
  let from = e.currentTarget.label; //XXX has this label to be the same across everything?
  if(e.data == "XXX") {
    log("XXX test received");
    return;
  }
  //log(e.data);
  let msg = JSON.parse(e.data);
    //log(msg);
    //if(msg.from != from) {
    // log('peer '+from+' trying to spoof '+msg.from+' msg ignored');
    //  return;
    //}
    let p = get_peer_by_name(msg.from); //XXX used to be from, but from is label
    if(!p) return null;
    switch(msg.type) {
      case 'unreachable': {
        log(msg.l2+' unreachable from '+msg.from);
        //XXX in JS is it better to compare strings or objects?
        if (l2_peers[msg.l2].gw.name == p.name) {
          delete l2_peers[msg.l2];
        }
        break;
      }
      case 'fwd': {
        log("fwd message...");
        if(msg.to == TID) {log("handle locally"); handle_l2_msg(msg, p); return;}
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
        msg.from = TID;
        msg.ori = p.alias;
        msg.to = dest_name;
        dest_peer.sendJSON(msg);
        break;
      }
      case 'search': {
        log(msg);
        //resp_google_search(p, msg.q);
        break;
      }
      case "l2": {
        //log(msg);
        let state = 'new';
        if(msg.i) state = 'initiator';
        for(l in msg.l2) {
          //log("L2 MSG received:", msg.l2[l], "gw:", p.name);
          //log("L2 L received:", l, "gw:", p.name);
          l2_peers[msg.l2[l]] = {
            name: msg.l2[l], 
            pk:'',
            gw:p, 
            state: state, 
            msg_ignored: 0
          };
        }
        get_all_keys();
        break;
      }
      case "extend": {
        log(msg);
        send_l2(p);
        break;
      }
      case "getl2": {
        log(msg);
        send_l2(p);
        break;
      }
      case 'resp': {
        log(msg);
        break;
      }
      case 'ping': {
        let ping_text = msg.text;
        let rep = {
          from: TID,
          type: 'pong',
          text: ping_text,
        }
        if(p.sendJSON) p.sendJSON(rep);
        else {log(p.name + "has not yet sendJSON but receive msgs")}
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
      default: {
        log(msg.type, "not implemented")
      }
    }
  }

  function delete_l1_peer(o) {
    delete l1_peers[o.name];
    if(o.dc) o.dc.close();
    o.pc.close();
    log(o.name+' L1 peer lost'); //XXX send to status
    status_log += o.name+' disconnected.\r\n';
    let l2lost = 0;
    for(k in l2_peers) {
      let l = l2_peers[k];
      if(l.gw.name == o.name) {
        delete l2_peers[k];
        l2lost++;
      }
    }
    if(l2lost) status_log += l2lost+' L2 peers lost.\r\n';
  }
  
