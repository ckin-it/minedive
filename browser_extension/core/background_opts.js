function apply_options() {
    chrome.storage.local.get(['options'], function(result) {
      let r = 0;
      let new_options = {};
      if(result['options']) new_options = result['options'];
      else new_options = default_options;
      if(options.ws_server != new_options.ws_server) r = 1;
      if(options.staticID != new_options.staticID) r = 1;
      if(options.community != new_options.community) r = 1;
      options = new_options;
      if(r) reconnect();
    });
  }
  