(function() {
    // Bookmarklet result popup: countdown and close
    var closeBtn = document.getElementById("bookmarklet-close");
    if (closeBtn) {
        closeBtn.addEventListener("click", function() { window.close(); });
    }

    var el = document.getElementById("seconds");
    if (el) {
        var s = 3;
        var timer = setInterval(function() {
            s--;
            el.textContent = s;
            if (s <= 0) {
                clearInterval(timer);
                window.close();
            }
        }, 1000);
    }

    // Main page: set bookmarklet link href using relative URL resolution
    var bookmarkletLink = document.getElementById("bookmarklet-link");
    if (bookmarkletLink) {
        var bookmarkletURL = new URL("./bookmarklet", location.href).href;
        bookmarkletLink.href = "javascript:void(window.open('" +
            bookmarkletURL +
            "?url='+encodeURIComponent(location.href),'linksaver','width=450,height=300'))";
    }
})();
