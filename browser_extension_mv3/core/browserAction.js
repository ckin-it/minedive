let ba = chrome.browserAction;

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
  