"use strict";

const POLL_INTERVAL_MS = 2000;
const POLL_TIMEOUT_MS = 5 * 60 * 1000;
const PROFILE_PREFIX = "https://steamcommunity.com/id/";

const form = document.querySelector("#lookup-form");
const input = document.querySelector("#steam-name");
const button = document.querySelector("#lookup-button");
const status = document.querySelector("#status");

const result = document.querySelector("#result");
const resultName = document.querySelector("#result-name");
const steamID = document.querySelector("#steam-id");
const resultSource = document.querySelector("#result-source");

const jobSection = document.querySelector("#job");
const jobStatus = document.querySelector("#job-status");
const jobProgress = document.querySelector("#job-progress");
const jobSkipped = document.querySelector("#job-skipped");
const jobFailed = document.querySelector("#job-failed");
const jobError = document.querySelector("#job-error");

let searchToken = 0;
let activeController = null;

form.addEventListener("submit", (event) => {
    event.preventDefault();
    void search();
});

async function search() {
    const token = ++searchToken;
    activeController?.abort();
    const controller = new AbortController();
    activeController = controller;

    const name = normalizeName(input.value);
    resetView();

    if (!name) {
        setStatus("Enter a Steam name.", "error");
        input.focus();
        return;
    }

    setBusy(true);
    setStatus(`Looking up “${name}”…`);

    try {
        const lookupResponse = await fetch(lookupURL(name), {
            headers: { Accept: "application/json" },
            signal: controller.signal,
        });
        if (!isCurrent(token)) {
            return;
        }

        if (lookupResponse.status === 200) {
            const body = await lookupResponse.json();
            if (!isCurrent(token)) {
                return;
            }
            showSteamUser(body.steamUser, "database");
            setStatus("Found in the database. No import needed.");
            return;
        }

        if (lookupResponse.status !== 404) {
            throw await readError(lookupResponse);
        }

        setStatus(`Not in the database. Resolving “${name}” with Steam…`);

        const createResponse = await fetch("/api/steamuser", {
            method: "POST",
            headers: {
                Accept: "application/json",
                "Content-Type": "application/json",
            },
            body: JSON.stringify({ vanityUrl: name }),
            signal: controller.signal,
        });
        if (!isCurrent(token)) {
            return;
        }

        if (createResponse.status === 404) {
            throw new Error("Steam user not found.");
        }
        if (!createResponse.ok) {
            throw await readError(createResponse);
        }

        const body = await createResponse.json();
        if (!isCurrent(token)) {
            return;
        }

        showSteamUser(body.steamUser, "steam");
        setStatus("User saved. Import started.");

        if (body.importJob) {
            startPolling(body.importJob, token, controller.signal);
        } else {
            setStatus("User saved. No import job was returned.", "error");
        }
    } catch (error) {
        if (isAbort(error) || !isCurrent(token)) {
            return;
        }
        setStatus(errorMessage(error), "error");
    } finally {
        if (isCurrent(token)) {
            setBusy(false);
        }
    }
}

function startPolling(initialJob, token, signal) {
    renderJob(initialJob);

    const deadline = Date.now() + POLL_TIMEOUT_MS;

    const poll = async () => {
        if (!isCurrent(token)) {
            return;
        }
        if (Date.now() > deadline) {
            setStatus(
                "The import is taking longer than expected. You can check back later.",
                "error",
            );
            return;
        }

        try {
            const response = await fetch(jobURL(initialJob.id), {
                headers: { Accept: "application/json" },
                signal,
            });
            if (!isCurrent(token)) {
                return;
            }

            if (response.status === 200) {
                const currentJob = await response.json();
                if (!isCurrent(token)) {
                    return;
                }
                renderJob(currentJob);
                if (
                    currentJob.status === "succeeded" ||
                    currentJob.status === "failed"
                ) {
                    return;
                }
            } else if (response.status === 404) {
                jobStatus.textContent = "Import job not found.";
                setStatus("Import job not found.", "error");
                return;
            } else {
                jobStatus.textContent = "Could not read import status; retrying…";
            }
        } catch (error) {
            if (isAbort(error) || !isCurrent(token)) {
                return;
            }
            jobStatus.textContent = "Could not read import status; retrying…";
        }

        setTimeout(poll, POLL_INTERVAL_MS);
    };

    void poll();
}

function renderJob(job) {
    if (!job) {
        return;
    }

    jobSection.hidden = false;
    jobStatus.textContent = jobStatusMessage(job);
    jobProgress.textContent = `${job.processedApps ?? 0} / ${job.totalApps ?? 0}`;
    jobSkipped.textContent = String(job.skippedApps ?? 0);
    jobFailed.textContent = String(job.failedApps ?? 0);

    if (job.error) {
        jobError.textContent = job.error;
        jobError.hidden = false;
    } else {
        jobError.textContent = "";
        jobError.hidden = true;
    }
}

function jobStatusMessage(job) {
    switch (job.status) {
        case "queued":
            return "Queued for import.";
        case "running":
            return "Importing your library…";
        case "succeeded":
            return "Import complete.";
        case "failed":
            return "Import failed.";
        default:
            return `Import status: ${job.status ?? "unknown"}.`;
    }
}

function showSteamUser(steamUser, source) {
    if (!steamUser) {
        setStatus("The server did not return a Steam user.", "error");
        return;
    }

    resultName.textContent = steamUser.name ?? "";
    // Steam IDs exceed JavaScript's safe integer range, so keep the value as
    // text and never convert it to a number.
    steamID.textContent =
        steamUser.steam_id === null || steamUser.steam_id === undefined
            ? ""
            : String(steamUser.steam_id);
    resultSource.textContent =
        source === "database" ? "Stored in the database" : "Resolved with Steam";
    result.hidden = false;
}

function resetView() {
    result.hidden = true;
    jobSection.hidden = true;

    resultName.textContent = "";
    steamID.textContent = "";
    resultSource.textContent = "";

    jobStatus.textContent = "";
    jobProgress.textContent = "";
    jobSkipped.textContent = "";
    jobFailed.textContent = "";
    jobError.textContent = "";
    jobError.hidden = true;

    setStatus("");
}

function setStatus(message, state) {
    status.textContent = message;
    if (state) {
        status.dataset.state = state;
    } else {
        delete status.dataset.state;
    }
}

function setBusy(busy) {
    input.disabled = busy;
    button.disabled = busy;
}

function normalizeName(rawValue) {
    let name = rawValue.trim();
    if (name.toLowerCase().startsWith(PROFILE_PREFIX)) {
        name = name.slice(PROFILE_PREFIX.length);
    }
    return name.replace(/\/+$/, "").trim();
}

function lookupURL(name) {
    return `/api/steamusers?name=${encodeURIComponent(name)}`;
}

function jobURL(id) {
    return `/api/import-jobs/${encodeURIComponent(id)}`;
}

async function readError(response) {
    try {
        const body = await response.json();
        if (body && typeof body.error === "string" && body.error !== "") {
            return new Error(body.error);
        }
    } catch {
        // Fall through to the generic message.
    }
    return new Error(`Request failed (HTTP ${response.status}).`);
}

function errorMessage(error) {
    if (error instanceof TypeError) {
        return "Could not reach the server. Try again.";
    }
    return error.message || "Something went wrong. Try again.";
}

function isCurrent(token) {
    return token === searchToken;
}

function isAbort(error) {
    return error?.name === "AbortError";
}
