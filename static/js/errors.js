document.addEventListener("htmx:responseError", function (evt) {
  const error = evt.detail.xhr.responseText.trim();

  const pre = document.createElement("pre");
  pre.classList.add("text-small", "my-0");
  pre.appendChild(document.createTextNode(error));

  const div = document.createElement("div");
  div.classList.add("toast", "toast-error");
  div.appendChild(pre);

  evt.detail.target.replaceChildren(div);
});

document.addEventListener("htmx:sendError", function (evt) {
  const error = "Network error, please reload page."

  const div = document.createElement("div");
  div.classList.add("toast", "toast-error");
  div.appendChild(document.createTextNode(error));

  evt.detail.target.replaceChildren(div);
});
