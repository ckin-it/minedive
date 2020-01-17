async function create_offer(pc, user) {
    log('initiating '+user);
    await pc.setLocalDescription(await pc.createOffer());
    pc.onicecandidate = ({candidate}) => {
      if (candidate) return;
      log("LOCAL DESCRIPTION");
      log(pc.localDescription.sdp)
      ws_send_sdp(user, 'offer', pc.localDescription.sdp);
    };
  }
  
  async function accept_offer(pc, user, offer_value) {
    log('accept offer '+user);
    log(offer_value);
    if (pc.signalingState != "stable") return;
    await pc.setRemoteDescription({type: "offer", sdp: offer_value});
    await pc.setLocalDescription(await pc.createAnswer());
    pc.onicecandidate = ({candidate}) => {
      if (candidate) return;
      log("LOCAL DESCRIPTION");
      log(pc.localDescription.sdp)
      ws_send_sdp(user, 'answer', pc.localDescription.sdp);
    };
  };
  
  function accept_answer(pc, answer_value) {
    if (pc.signalingState != "have-local-offer") return;
    pc.setRemoteDescription({type: "answer", sdp: answer_value});
  };
  
