(function () {
    "use strict";

    const TTS_BASE_URL = "http://localhost:8090";
    const TTS_TOKEN = "";

    function entryIDFromStarURL(starURL) {
        const match = String(starURL || "").match(/\/entry\/star\/(\d+)/);
        return match ? match[1] : "";
    }

    function setButtonState(button, state) {
        button.dataset.ttsState = state;
        if (state === "loading") {
            button.disabled = true;
            button.textContent = "TTS...";
        } else if (state === "done") {
            button.disabled = false;
            button.textContent = "TTS";
        } else {
            button.disabled = false;
            button.textContent = "TTS";
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

    function buildButton(entryID, starButton) {
        const button = document.createElement("button");
        button.type = "button";
        button.className = starButton.className || "page-button";
        button.dataset.ttsEntryID = entryID;
        button.title = "Generate TTS audio";
        button.textContent = "TTS";
        button.addEventListener("click", function (event) {
            event.preventDefault();
            requestTTS(entryID, button);
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
            if (!starItem || starItem.nextElementSibling && starItem.nextElementSibling.dataset.ttsItem === entryID) {
                return;
            }

            const item = document.createElement("li");
            item.dataset.ttsItem = entryID;
            item.appendChild(buildButton(entryID, starButton));
            starItem.insertAdjacentElement("afterend", item);
        });
    }

    insertButtons();
    new MutationObserver(insertButtons).observe(document.body, { childList: true, subtree: true });
})();
