async function create_offer(pc, user) {
    //log('initiating '+user);
    await pc.setLocalDescription(await pc.createOffer());
    pc.onicecandidate = ({candidate}) => {
      //if (candidate) log("OnICECandidate (ao)"+candidate.address);
      if (candidate) return;
      //null candidate means I've got them all
      ws_send_sdp(user, 'offer', pc.localDescription.sdp);
      //log("OFFER XXX SDP:")
      //log(pc.localDescription.sdp);
    };
  }
  
  async function accept_offer(pc, user, offer_value) {
    //log('accept offer '+user);
    if (pc.signalingState != "stable") return;
    //log(offer_value);
    await pc.setRemoteDescription({type: "offer", sdp: offer_value});
    await pc.setLocalDescription(await pc.createAnswer());
    pc.onicecandidate = ({candidate}) => {
      //if (candidate) log("OnICECandidate (ao)"+candidate.address);
      if (candidate) return;
      //null candidate means I've got them all
      //log("ANSWER XXX SDP:")
      //log(pc.localDescription.sdp);
      ws_send_sdp(user, 'answer', pc.localDescription.sdp);
    };
    status_log += 'L1 '+user+' offer accepted\r\n';
  };
  
  function accept_answer(pc, answer_value) {
    if (pc.signalingState != "have-local-offer") return;
    pc.setRemoteDescription({type: "answer", sdp: answer_value});
    status_log += 'L1 answer accepted\r\n';
  };
  
