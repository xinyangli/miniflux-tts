(function () {
    "use strict";

    const TTS_BASE_URL = "http://localhost:8090";
    const TTS_TOKEN = "";

    function entryIDFromStarURL(starURL) {
        const match = String(starURL || "").match(/\/entry\/star\/(\d+)/);
        return match ? match[1] : "";
    }

    function audioBaseURL() {
        return `${TTS_BASE_URL.replace(/\/$/, "")}/audio/`;
    }

    function isTTSURL(value) {
        return String(value || "").startsWith(audioBaseURL());
    }

    function hasTTSResult(entryID) {
        const entryPage = document.querySelector("section.entry[data-id]");
        if (!entryPage || entryPage.dataset.id !== String(entryID)) {
            return false;
        }

        return Array.from(document.querySelectorAll("a[href], source[src], audio[src]")).some(function (element) {
            return isTTSURL(element.getAttribute("href")) ||
                isTTSURL(element.href) ||
                isTTSURL(element.getAttribute("src")) ||
                isTTSURL(element.src);
        });
    }

    function buttonMode(entryID) {
        return hasTTSResult(entryID) ? "delete" : "generate";
    }

    function setButtonState(button, state, mode) {
        mode = mode || button.dataset.ttsMode || "generate";
        button.dataset.ttsState = state;
        if (state === "loading") {
            button.disabled = true;
            button.textContent = mode === "delete" ? "Deleting..." : "TTS...";
        } else if (state === "done") {
            button.disabled = false;
            button.textContent = mode === "delete" ? "Delete TTS" : "TTS";
        } else {
            button.disabled = false;
            button.textContent = mode === "delete" ? "Delete TTS" : "TTS";
        }
    }

    async function requestTTS(entryID, button) {
        setButtonState(button, "loading");
        try {
            const headers = {};
            if (TTS_TOKEN) {
                headers["X-TTS-Token"] = TTS_TOKEN;
            }
            const response = await fetch(`${TTS_BASE_URL.replace(/\/$/, "")}/tts/${entryID}`, {
                method: "POST",
                headers,
            });
            if (!response.ok) {
                throw new Error(`TTS request failed: ${response.status}`);
            }
            setButtonState(button, "done");
        } catch (error) {
            console.error(error);
            button.disabled = false;
            button.textContent = "TTS!";
        }
    }

    async function deleteTTS(entryID, button) {
        setButtonState(button, "loading", "delete");
        try {
            const headers = {};
            if (TTS_TOKEN) {
                headers["X-TTS-Token"] = TTS_TOKEN;
            }
            const response = await fetch(`${TTS_BASE_URL.replace(/\/$/, "")}/tts/${entryID}`, {
                method: "DELETE",
                headers,
            });
            if (!response.ok) {
                throw new Error(`TTS delete failed: ${response.status}`);
            }
            window.location.reload();
        } catch (error) {
            console.error(error);
            button.disabled = false;
            button.textContent = "Delete!";
        }
    }

    function buildButton(entryID, starButton, mode) {
        const button = document.createElement("button");
        button.type = "button";
        button.className = starButton.className || "page-button";
        button.dataset.ttsEntryID = entryID;
        button.dataset.ttsMode = mode;
        button.title = mode === "delete" ? "Delete TTS audio" : "Generate TTS audio";
        button.textContent = mode === "delete" ? "Delete TTS" : "TTS";
        button.addEventListener("click", function (event) {
            event.preventDefault();
            if (button.dataset.ttsMode === "delete") {
                deleteTTS(entryID, button);
            } else {
                requestTTS(entryID, button);
            }
        });
        return button;
    }

    function insertButtons() {
        document.querySelectorAll("[data-star-url]").forEach(function (starButton) {
            const entryID = entryIDFromStarURL(starButton.dataset.starUrl);
            if (!entryID) {
                return;
            }

            const starItem = starButton.closest("li");
            if (!starItem) {
                return;
            }

            const existingItem = starItem.nextElementSibling && starItem.nextElementSibling.dataset.ttsItem === entryID
                ? starItem.nextElementSibling
                : null;
            const mode = buttonMode(entryID);
            const existingButton = existingItem?.querySelector("button");
            if (existingButton && existingButton.dataset.ttsMode === mode) {
                return;
            }
            existingItem?.remove();

            const item = document.createElement("li");
            item.dataset.ttsItem = entryID;
            item.appendChild(buildButton(entryID, starButton, mode));
            starItem.insertAdjacentElement("afterend", item);
        });
    }

    insertButtons();
    new MutationObserver(insertButtons).observe(document.body, { childList: true, subtree: true });
})();
