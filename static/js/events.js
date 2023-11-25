document.addEventListener("htmx:beforeRequest", function (evt) {
  const btn = evt.detail.elt.querySelector("button.input-group-btn");
  btn.classList.add("loading");
  btn.disabled = true;
});

document.addEventListener("htmx:afterRequest", function (evt) {
  const btn = evt.detail.elt.querySelector("button.input-group-btn");
  btn.classList.remove("loading");
  btn.disabled = false;
});

document.addEventListener("htmx:responseError", function (evt) {
  const error = evt.detail.xhr.responseText.trim();

  const pre = document.createElement("pre");
  pre.classList.add("response-error");
  pre.appendChild(document.createTextNode(error));

  const div = document.createElement("div");
  div.classList.add("toast", "toast-error");
  div.appendChild(pre);

  evt.detail.target.replaceChildren(div);
});

document.addEventListener("htmx:sendError", function (evt) {
  const error = "Network error, please try again or reload page."

  const div = document.createElement("div");
  div.classList.add("toast", "toast-error");
  div.appendChild(document.createTextNode(error));

  evt.detail.target.replaceChildren(div);
});
