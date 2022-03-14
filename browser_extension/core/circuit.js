class Circuit {
  //dc dataChannel with the first peer
  //cfg is the CircuitConfiguration
  //[]peers
  //[]query
  //state

  constructor(peer, cfg) {
    //ask for a direct peer
    if (cfg.multi) {
      //throw error!
    }
    //establish datachannel
    this.dc = peer.dc;
    this.peers = new Array();
    this.peers[0] = peer;
    //create ping on the circuit
  }

  extend() {
    log("do something to extend your circuit");
    this.peers.slice().reverse().forEach(x => console.log(x));
    let msg = {
      type: 'extend',
      d0: 'something',
    }
    this.dc.send(JSON.stringify(msg));
    //let peer = new Peer();
    //this.peers.push(peer);
  }

  search(q) {

  }
}

class Peer {
  //name
  //key
}

class Query {
  //q is the query
  //r is results
}

class CircuitConfiguration {
  //unique server or multiple server
  //server, {s,k,p}_server
  constructor() {
    this.multi = false;
    //this.server = default_options.ws_server;
    //this.s_server = default_options.ws_server;
    //this.k_server = default_options.ws_server;
    //this.p_server = default_options.ws_server;
  }
}
