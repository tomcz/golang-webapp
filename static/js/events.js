document.addEventListener("htmx:beforeRequest", function (evt) {
  const fieldset = evt.detail.elt.querySelector("fieldset");
  fieldset.disabled = true;
  const btn = evt.detail.elt.querySelector("button.input-group-btn");
  btn.classList.add("loading");
});

document.addEventListener("htmx:afterRequest", function (evt) {
  const fieldset = evt.detail.elt.querySelector("fieldset");
  fieldset.disabled = false;
  const btn = evt.detail.elt.querySelector("button.input-group-btn");
  btn.classList.remove("loading");
});

document.addEventListener("htmx:responseError", function (evt) {
  const div = document.createElement("div");
  div.innerText = evt.detail.xhr.responseText.trim()
  div.classList.add("toast", "toast-error");
  evt.detail.target.replaceChildren(div);
});

document.addEventListener("htmx:sendError", function (evt) {
  const div = document.createElement("div");
  div.innerText = "Network error, please try again or reload page.";
  div.classList.add("toast", "toast-error");
  evt.detail.target.replaceChildren(div);
});
