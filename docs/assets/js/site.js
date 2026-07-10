(function () {
  "use strict";

  function fallbackCopy(text) {
    var input = document.createElement("textarea");
    input.value = text;
    input.setAttribute("readonly", "");
    input.style.position = "fixed";
    input.style.opacity = "0";
    document.body.appendChild(input);
    input.select();
    var copied = document.execCommand("copy");
    input.remove();
    if (!copied) throw new Error("copy command was rejected");
  }

  window.copyInstallCommand = async function (button) {
    var container = button.closest(".command-copy");
    var code = container && container.querySelector("code");
    if (!code) return;

    var command = code.textContent.trim();
    var originalLabel = button.dataset.originalLabel || button.textContent;
    button.dataset.originalLabel = originalLabel;

    try {
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(command);
      } else {
        fallbackCopy(command);
      }
      button.textContent = "Copied!";
      button.classList.add("copied");
    } catch (_) {
      button.textContent = "Copy failed";
      button.classList.remove("copied");
    }

    window.clearTimeout(button.copyResetTimer);
    button.copyResetTimer = window.setTimeout(function () {
      button.textContent = originalLabel;
      button.classList.remove("copied");
    }, 1800);
  };
})();
