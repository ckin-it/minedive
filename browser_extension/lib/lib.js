let log = console.log;

function inc_nonce(nonce, dyn) {
  for(let i = 1; i <= dyn; i++)
  {
    if(nonce[nonce.length-i] < 0xff) {
      nonce[nonce.length-i]++;
      return true;
    } else {
      nonce[nonce.length-i] = 0;
    }
  }
  return false;
}

function makeid(length) {
  let result = '';
  let caps = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
  let characters = '_abcdefghijklmnopqrstuvwxyz0123456789';
  let charactersLength = characters.length;
  result = caps.charAt(Math.floor(Math.random() * caps.length));
  for (let i = 0; i < length-1; i++ ) {
     result += characters.charAt(Math.floor(Math.random() * characters.length));
  }
  return result;
}
 