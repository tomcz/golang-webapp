document.querySelectorAll("div.toast button.btn-clear").forEach((btn) => {
  btn.addEventListener("click", () => {
    btn.closest("div.toast").classList.add("d-hide");
    return false;
  });
});
