function report_peers() {
  log("aliases");
  log(alias_to_peers);
  log("L1");
  log(l1_peers);
  log("L2");
  log(l2_peers);
}

function emptyDB() {
  chrome.storage.local.clear(function(){bkg.console.log('data clear');});
}
  
function dumpDB() {
  chrome.storage.local.get(null, function(items) {
    var allKeys = Object.keys(items);
    console.log(allKeys);
  });
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
