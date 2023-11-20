document.querySelectorAll("div.toast button.btn-clear").forEach((btn) => {
  btn.addEventListener("click", () => {
    btn.closest("div.toast").style.display = "none";
    return false;
  });
});
