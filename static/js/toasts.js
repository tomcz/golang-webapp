document.querySelectorAll("div.toast button.btn-clear")
  .forEach(function (btn) {
    btn.addEventListener("click", function () {
      btn.closest("div.toast").style.display = "none";
      return false;
    });
  });
