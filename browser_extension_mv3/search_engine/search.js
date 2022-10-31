async function search(l2, q, l, key, nonce) {
  //await arxiv_resp_search_l2(l2, q, l, key, nonce);
  await resp_google_search_l2(l2, q, l, key, nonce);
}

function makeRequest(method, url, h) {
  return new Promise(function (resolve, reject) {
      let xhr = new XMLHttpRequest();
      xhr.open(method, url);
      xhr.onload = function () {
          if (this.status >= 200 && this.status < 300) {
              let r = h(xhr.response);
              resolve(r);
          } else {
              reject({
                  status: this.status,
                  statusText: xhr.statusText
              });
          }
      };
      xhr.onerror = function () {
          reject({
              status: this.status,
              statusText: xhr.statusText
          });
      };
      xhr.send();
  });
}