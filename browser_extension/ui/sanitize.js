var sanitizeHTML = function (str) {
	var tmp = document.createElement('div');
	tmp.textContent = str;
	return tmp.innerHTML;
};
