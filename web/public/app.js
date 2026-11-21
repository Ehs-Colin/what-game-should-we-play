"use strict";

const POLL_INTERVAL_MS = 2000;
const POLL_TIMEOUT_MS = 5 * 60 * 1000;
const DASHBOARD_REFRESH_MS = 5000;
const PROFILE_PREFIX = "https://steamcommunity.com/id/";

const MAX_COMMON_GAME_USERS = 25;
const COMMON_GAMES_LIMIT_MIN = 1;
const COMMON_GAMES_LIMIT_MAX = 100;
const COMMON_GAMES_LIMIT_DEFAULT = 10;

const ROUTES = new Map([
    ["/", "dashboard"],
    ["/import", "import"],
    ["/search", "search"],
    ["/user", "user"],
    ["/common-games", "commonGames"],
    ["/login", "login"],
    ["/create-account", "createAccount"],
]);

const ROUTE_TITLES = {
    dashboard: "Dashboard",
    import: "Import Steam User",
    search: "Search Users",
    user: "View User",
    commonGames: "Find Common Games",
    login: "Sign in",
    createAccount: "Create Account",
};

const authState = {
    loaded: false,
    authenticated: false,
    user: null,
    csrfToken: null,
    mustCreateAccount: false,
};

const viewSections = document.querySelectorAll("[data-view]");
const navLinks = document.querySelectorAll(".site-nav a[data-route]");
const siteNav = document.querySelector("#site-nav");
const authUsername = document.querySelector("#auth-username");
const logoutButton = document.querySelector("#logout-button");

const loginForm = document.querySelector("#login-form");
const loginUsername = document.querySelector("#login-username");
const loginPassword = document.querySelector("#login-password");
const loginButton = document.querySelector("#login-button");
const loginStatus = document.querySelector("#login-status");

const createAccountForm = document.querySelector("#create-account-form");
const createUsername = document.querySelector("#create-username");
const createPassword = document.querySelector("#create-password");
const createPasswordConfirm = document.querySelector(
    "#create-password-confirm",
);
const createDefaultPassword = document.querySelector(
    "#create-default-password",
);
const createAccountButton = document.querySelector("#create-account-button");
const createAccountStatus = document.querySelector("#create-account-status");

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

const dashboardStatus = document.querySelector("#dashboard-status");
const dashboardMetrics = document.querySelector("#dashboard-metrics");
const dashboardUpdated = document.querySelector("#dashboard-updated");
const metricUsers = document.querySelector("#metric-users");
const metricApps = document.querySelector("#metric-apps");
const metricRunning = document.querySelector("#metric-running");
const metricQueued = document.querySelector("#metric-queued");

const searchForm = document.querySelector("#search-form");
const searchInput = document.querySelector("#search-name");
const searchButton = document.querySelector("#search-button");
const searchControls = document.querySelector("#search-controls");
const searchSort = document.querySelector("#search-sort");
const searchDirection = document.querySelector("#search-direction");
const searchWishlist = document.querySelector("#search-wishlist");
const searchReset = document.querySelector("#search-reset");
const searchStatus = document.querySelector("#search-status");
const searchCount = document.querySelector("#search-count");
const searchResultsPanel = document.querySelector("#search-results-panel");
const searchResults = document.querySelector("#search-results");

const userForm = document.querySelector("#user-form");
const userInput = document.querySelector("#user-name");
const userStatus = document.querySelector("#user-status");
const userSummary = document.querySelector("#user-summary");
const userStatusBadge = document.querySelector("#user-status-badge");
const userSummaryDetails = document.querySelector("#user-summary-details");
const userSummaryNotice = document.querySelector("#user-summary-notice");
const userApps = document.querySelector("#user-apps");
const tabOwned = document.querySelector("#tab-owned");
const tabWishlist = document.querySelector("#tab-wishlist");
const ownedTabCount = document.querySelector("#owned-tab-count");
const wishlistTabCount = document.querySelector("#wishlist-tab-count");
const userAppPanel = document.querySelector("#user-app-panel");
const userListCount = document.querySelector("#user-list-count");
const userPageSize = document.querySelector("#user-page-size");
const userAppRows = document.querySelector("#user-app-rows");
const userPagination = document.querySelector("#user-pagination");

const commonGamesSearchForm = document.querySelector(
    "#common-games-search-form",
);
const commonGamesSearchInput = document.querySelector(
    "#common-games-search-name",
);
const commonGamesSearchButton = document.querySelector(
    "#common-games-search-button",
);
const commonGamesSearchStatus = document.querySelector(
    "#common-games-search-status",
);
const commonGamesSearchResultsPanel = document.querySelector(
    "#common-games-search-results-panel",
);
const commonGamesSearchResults = document.querySelector(
    "#common-games-search-results",
);
const commonGamesSelectedCount = document.querySelector(
    "#common-games-selected-count",
);
const commonGamesSelectedEmpty = document.querySelector(
    "#common-games-selected-empty",
);
const commonGamesSelectedList = document.querySelector(
    "#common-games-selected-list",
);
const commonGamesSelectedHint = document.querySelector(
    "#common-games-selected-hint",
);
const commonGamesLimit = document.querySelector("#common-games-limit");
const commonGamesSubmit = document.querySelector("#common-games-submit");
const commonGamesSubmitHint = document.querySelector(
    "#common-games-submit-hint",
);
const commonGamesStatus = document.querySelector("#common-games-status");
const commonGamesResults = document.querySelector("#common-games-results");
const commonGamesResultsCount = document.querySelector(
    "#common-games-results-count",
);
const commonGamesQueryUsers = document.querySelector(
    "#common-games-query-users",
);
const commonGamesResultsEmpty = document.querySelector(
    "#common-games-results-empty",
);
const commonGamesTableScroll = document.querySelector(
    "#common-games-table-scroll",
);
const commonGamesGameRows = document.querySelector("#common-games-game-rows");
const commonGamesNote = document.querySelector("#common-games-note");

const numberFormatter = new Intl.NumberFormat();

let currentRoute = null;
let currentHref = null;

let importToken = 0;
let lookupController = null;
let lookupBusy = false;
let pollTimer = null;
let pollController = null;
let lastJob = null;
let pollDeadline = 0;

let dashboardTimer = null;
let dashboardController = null;

let searchToken = 0;
let searchController = null;
let searchBusy = false;
let searchUsers = [];

const SEARCH_SORT_DEFAULT_DIRECTIONS = {
    name: "asc",
    updated: "desc",
    owned: "desc",
    wishlist: "desc",
};

const APP_PAGE_SIZES = [25, 50, 100];

let userToken = 0;
let userController = null;
const userState = {
    name: "",
    list: "owned",
    page: 1,
    pageSize: 25,
    pages: { owned: 1, wishlist: 1 },
    data: null,
};

let commonGamesSearchToken = 0;
let commonGamesSearchController = null;
let commonGamesSearchBusy = false;

let commonGamesRequestToken = 0;
let commonGamesRequestController = null;
let commonGamesBusy = false;

let commonGamesUsersToken = 0;
let commonGamesUsersController = null;

const commonGamesState = {
    selectedUsers: new Map(),
    searchUsers: [],
    results: null,
    submitted: false,
};

form.addEventListener("submit", (event) => {
    event.preventDefault();
    void search();
});

searchForm.addEventListener("submit", (event) => {
    event.preventDefault();
    void performSearch();
});

searchSort.addEventListener("change", () => {
    searchDirection.value =
        SEARCH_SORT_DEFAULT_DIRECTIONS[searchSort.value] ?? "asc";
    renderSearchPipeline();
});
searchDirection.addEventListener("change", renderSearchPipeline);
searchWishlist.addEventListener("change", renderSearchPipeline);
searchReset.addEventListener("click", () => {
    resetSearchControls();
    renderSearchPipeline();
});

userForm.addEventListener("submit", (event) => {
    event.preventDefault();
    const name = userInput.value.trim();
    if (!name) {
        setUserStatus("Enter a Steam name.", "error");
        userInput.focus();
        return;
    }
    userState.pages = { owned: 1, wishlist: 1 };
    setActiveTab("owned");
    pushUserURL(name, "owned", 1, userPageSizeFromValue(userPageSize.value));
});

tabOwned.addEventListener("click", () => {
    selectUserList("owned");
});
tabWishlist.addEventListener("click", () => {
    selectUserList("wishlist");
});
userPageSize.addEventListener("change", () => {
    const pageSize = userPageSizeFromValue(userPageSize.value);
    userState.pages = { owned: 1, wishlist: 1 };
    if (!userState.name) {
        userState.pageSize = pageSize;
        return;
    }
    pushUserURL(userState.name, userState.list, 1, pageSize);
});

commonGamesSearchForm.addEventListener("submit", (event) => {
    event.preventDefault();
    void performCommonGamesSearch();
});

commonGamesSubmit.addEventListener("click", () => {
    void findCommonGames();
});

commonGamesLimit.addEventListener("input", () => {
    invalidateCommonGamesResults();
});

loginForm.addEventListener("submit", (event) => {
    event.preventDefault();
    void login();
});

createAccountForm.addEventListener("submit", (event) => {
    event.preventDefault();
    void createRealAccount();
});

logoutButton.addEventListener("click", () => {
    void logout();
});

commonGamesSearchResults.addEventListener("click", (event) => {
    const target = event.target.closest("button[data-add-steam-id]");
    if (!target) {
        return;
    }
    const steamID = target.dataset.addSteamId ?? "";
    const user = commonGamesState.searchUsers.find(
        (candidate) => String(candidate.steamId) === steamID,
    );
    if (user) {
        addCommonGamesUser(user);
    }
});

commonGamesSelectedList.addEventListener("click", (event) => {
    const target = event.target.closest("button[data-remove-steam-id]");
    if (!target) {
        return;
    }
    removeCommonGamesUser(target.dataset.removeSteamId ?? "");
});

async function search() {
    const token = ++importToken;
    lookupController?.abort();
    stopPolling();
    lastJob = null;

    const controller = new AbortController();
    lookupController = controller;

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
        const lookupResponse = await apiFetch(lookupURL(name), {
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

        const createResponse = await apiFetch("/api/steamuser", {
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
            startPolling(body.importJob, token);
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
            lookupController = null;
        }
    }
}

function startPolling(job, token) {
    lastJob = job;
    pollDeadline = Date.now() + POLL_TIMEOUT_MS;
    renderJob(job);

    if (onImportRoute()) {
        void pollOnce(token);
    }
}

async function pollOnce(token) {
    if (token !== importToken || !onImportRoute() || !lastJob) {
        return;
    }
    if (Date.now() > pollDeadline) {
        setStatus(
            "The import is taking longer than expected. You can check back later.",
            "error",
        );
        return;
    }

    pollController?.abort();
    const controller = new AbortController();
    pollController = controller;
    let shouldContinue = true;

    try {
        const response = await apiFetch(jobURL(lastJob.id), {
            headers: { Accept: "application/json" },
            signal: controller.signal,
        });
        if (token !== importToken || !onImportRoute()) {
            return;
        }

        if (response.status === 200) {
            const currentJob = await response.json();
            if (token !== importToken || !onImportRoute()) {
                return;
            }
            lastJob = currentJob;
            renderJob(currentJob);
            if (
                currentJob.status === "succeeded" ||
                currentJob.status === "failed"
            ) {
                shouldContinue = false;
            }
        } else if (response.status === 404) {
            jobStatus.textContent = "Import job not found.";
            setStatus("Import job not found.", "error");
            lastJob = null;
            shouldContinue = false;
        } else {
            jobStatus.textContent = "Could not read import status; retrying…";
        }
    } catch (error) {
        if (isAbort(error) || token !== importToken || !onImportRoute()) {
            return;
        }
        jobStatus.textContent = "Could not read import status; retrying…";
    } finally {
        if (pollController === controller) {
            pollController = null;
        }
    }

    if (shouldContinue && token === importToken && onImportRoute()) {
        pollTimer = setTimeout(() => {
            pollTimer = null;
            void pollOnce(token);
        }, POLL_INTERVAL_MS);
    }
}

function stopPolling() {
    if (pollTimer !== null) {
        clearTimeout(pollTimer);
        pollTimer = null;
    }
    pollController?.abort();
    pollController = null;
}

function enterImport() {
    if (
        lastJob &&
        (lastJob.status === "queued" || lastJob.status === "running")
    ) {
        void pollOnce(importToken);
    }
}

function leaveImport() {
    lookupController?.abort();
    if (lookupBusy) {
        setBusy(false);
        setStatus("");
    }
    stopPolling();
}

function onImportRoute() {
    return currentRoute === "import";
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
        steamUser.steamId === null || steamUser.steamId === undefined
            ? ""
            : String(steamUser.steamId);
    resultSource.textContent =
        source === "database"
            ? "Stored in the database"
            : "Resolved with Steam";
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
    lookupBusy = busy;
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
    return token === importToken;
}

function isAbort(error) {
    return error?.name === "AbortError";
}

async function apiFetch(url, options = {}) {
    const headers = new Headers(options.headers || {});
    const method = (options.method || "GET").toUpperCase();
    if (!["GET", "HEAD", "OPTIONS"].includes(method) && authState.csrfToken) {
        headers.set("X-CSRF-Token", authState.csrfToken);
    }

    const response = await fetch(url, {
        ...options,
        headers,
        credentials: "same-origin",
    });

    if (response.status === 401) {
        clearAuthState();
        authState.loaded = true;
        const next = encodeURIComponent(location.pathname + location.search);
        history.replaceState({}, "", `/login?next=${next}`);
        updateAuthChrome();
        renderRoute();
        throw new Error("Authentication required.");
    }

    if (response.status === 403) {
        const body = await response
            .clone()
            .json()
            .catch(() => null);
        if (body?.error === "account creation required") {
            history.replaceState({}, "", "/create-account");
            updateAuthChrome();
            renderRoute();
            throw new Error(body.error);
        }
    }

    return response;
}

async function refreshSession() {
    try {
        const response = await fetch("/api/auth/session", {
            headers: { Accept: "application/json" },
            credentials: "same-origin",
        });
        if (!response.ok) {
            throw await readError(response);
        }
        applyAuthSession(await response.json());
    } catch {
        clearAuthState();
    } finally {
        authState.loaded = true;
    }
}

function applyAuthSession(session) {
    authState.authenticated = Boolean(session?.authenticated);
    authState.user = session?.user ?? null;
    authState.csrfToken = session?.csrfToken ?? null;
    authState.mustCreateAccount = Boolean(session?.mustCreateAccount);
}

function applyLoginResult(result) {
    authState.loaded = true;
    authState.authenticated = true;
    authState.user = result?.user ?? null;
    authState.csrfToken = result?.csrfToken ?? null;
    authState.mustCreateAccount = Boolean(result?.mustCreateAccount);
}

function clearAuthState() {
    authState.authenticated = false;
    authState.user = null;
    authState.csrfToken = null;
    authState.mustCreateAccount = false;
}

function updateAuthChrome() {
    if (authState.authenticated && authState.user) {
        authUsername.textContent = authState.mustCreateAccount
            ? `${authState.user.username} · Setup required`
            : authState.user.username;
    } else {
        authUsername.textContent = "";
    }
    logoutButton.hidden = !authState.authenticated;

    const path = normalizePath(location.pathname);
    siteNav.hidden = path === "/login" || path === "/create-account";
}

function authRedirectFor(routePath) {
    if (!authState.authenticated) {
        if (routePath === "/login") {
            return null;
        }
        const next = location.pathname + location.search + location.hash;
        return `/login?next=${encodeURIComponent(next)}`;
    }
    if (authState.mustCreateAccount) {
        return routePath === "/create-account" ? null : "/create-account";
    }
    if (routePath === "/login" || routePath === "/create-account") {
        return "/";
    }
    return null;
}

function nextRouteAfterLogin() {
    const next = new URLSearchParams(location.search).get("next");
    if (!next || !next.startsWith("/") || next.startsWith("//")) {
        return "/";
    }
    return next;
}

function setLoginStatus(message, state) {
    loginStatus.textContent = message;
    if (state) {
        loginStatus.dataset.state = state;
    } else {
        delete loginStatus.dataset.state;
    }
}

function setLoginBusy(busy) {
    loginUsername.disabled = busy;
    loginPassword.disabled = busy;
    loginButton.disabled = busy;
}

async function login() {
    const username = loginUsername.value.trim();
    const password = loginPassword.value;

    if (!username || !password) {
        setLoginStatus("Enter your username and password.", "error");
        (username ? loginPassword : loginUsername).focus();
        return;
    }

    const next = nextRouteAfterLogin();
    setLoginBusy(true);
    setLoginStatus("Signing in…");

    try {
        const response = await fetch("/api/auth/login", {
            method: "POST",
            headers: {
                Accept: "application/json",
                "Content-Type": "application/json",
            },
            credentials: "same-origin",
            body: JSON.stringify({ username, password }),
        });

        if (response.status === 401) {
            setLoginStatus("Invalid username or password.", "error");
            return;
        }
        if (response.status === 429) {
            setLoginStatus("Too many attempts. Try again shortly.", "error");
            return;
        }
        if (!response.ok) {
            throw await readError(response);
        }

        const body = await response.json();
        applyLoginResult(body);
        loginForm.reset();

        const target = authState.mustCreateAccount ? "/create-account" : next;
        history.replaceState({}, "", target);
        updateAuthChrome();
        renderRoute();
    } catch (error) {
        setLoginStatus(errorMessage(error), "error");
    } finally {
        setLoginBusy(false);
    }
}

function setCreateAccountStatus(message, state) {
    createAccountStatus.textContent = message;
    if (state) {
        createAccountStatus.dataset.state = state;
    } else {
        delete createAccountStatus.dataset.state;
    }
}

function setCreateAccountBusy(busy) {
    createUsername.disabled = busy;
    createPassword.disabled = busy;
    createPasswordConfirm.disabled = busy;
    createDefaultPassword.disabled = busy;
    createAccountButton.disabled = busy;
}

async function createRealAccount() {
    const username = createUsername.value.trim();
    const password = createPassword.value;
    const confirmation = createPasswordConfirm.value;
    const defaultPassword = createDefaultPassword.value;

    if (!username || !password || !confirmation || !defaultPassword) {
        setCreateAccountStatus("Fill in every field.", "error");
        return;
    }
    if (password !== confirmation) {
        setCreateAccountStatus("The new passwords do not match.", "error");
        createPasswordConfirm.focus();
        return;
    }

    setCreateAccountBusy(true);
    setCreateAccountStatus("Creating your account…");

    try {
        const response = await apiFetch("/api/auth/account", {
            method: "POST",
            headers: {
                Accept: "application/json",
                "Content-Type": "application/json",
            },
            body: JSON.stringify({ defaultPassword, username, password }),
        });
        if (!response.ok) {
            throw await readError(response);
        }

        const body = await response.json();
        applyLoginResult(body);
        createAccountForm.reset();
        history.replaceState({}, "", "/");
        updateAuthChrome();
        renderRoute();
    } catch (error) {
        if (isAbort(error)) {
            return;
        }
        setCreateAccountStatus(errorMessage(error), "error");
    } finally {
        setCreateAccountBusy(false);
    }
}

async function logout() {
    logoutButton.disabled = true;
    try {
        await apiFetch("/api/auth/logout", {
            method: "POST",
            headers: { Accept: "application/json" },
        });
    } catch {
        // Clear local state even when the session was already gone.
    } finally {
        logoutButton.disabled = false;
    }

    clearAuthState();
    authState.loaded = true;
    history.replaceState({}, "", "/login");
    updateAuthChrome();
    renderRoute();
}

async function bootstrap() {
    await refreshSession();
    updateAuthChrome();
    renderRoute();
}

async function performSearch() {
    const token = ++searchToken;
    searchController?.abort();
    const controller = new AbortController();
    searchController = controller;

    const name = searchInput.value.trim();
    if (!name) {
        clearSearchResults();
        setSearchStatus("Enter a Steam name.", "error");
        searchInput.focus();
        return;
    }

    setSearchBusy(true);
    setSearchStatus(`Searching for “${name}”…`);

    try {
        const response = await apiFetch(
            `/api/steamusers/search?name=${encodeURIComponent(name)}`,
            {
                headers: { Accept: "application/json" },
                signal: controller.signal,
            },
        );
        if (!isSearchCurrent(token)) {
            return;
        }
        if (!response.ok) {
            throw await readError(response);
        }

        const body = await response.json();
        if (!isSearchCurrent(token)) {
            return;
        }

        searchUsers = Array.isArray(body.steamUsers) ? body.steamUsers : [];
        resetSearchControls();

        if (searchUsers.length === 0) {
            clearSearchResults();
            setSearchStatus(`No users match “${name}”.`);
            return;
        }

        searchControls.hidden = false;
        setSearchStatus("");
        renderSearchPipeline();
    } catch (error) {
        if (isAbort(error) || !isSearchCurrent(token)) {
            return;
        }
        clearSearchResults();
        setSearchStatus(errorMessage(error), "error");
    } finally {
        if (isSearchCurrent(token)) {
            setSearchBusy(false);
            searchController = null;
        }
    }
}

function clearSearchResults() {
    searchUsers = [];
    searchResults.replaceChildren();
    searchResultsPanel.hidden = true;
    searchControls.hidden = true;
    searchCount.textContent = "";
    searchCount.hidden = true;
}

function renderSearchPipeline() {
    if (searchUsers.length === 0) {
        searchResultsPanel.hidden = true;
        searchControls.hidden = true;
        return;
    }

    searchControls.hidden = false;
    const visible = visibleSearchUsers();

    searchResults.replaceChildren(
        ...visible.flatMap((user, index) => buildSearchResultRows(user, index)),
    );
    searchResultsPanel.hidden = visible.length === 0;

    const hiddenCount = searchUsers.length - visible.length;
    if (hiddenCount > 0) {
        searchCount.textContent = `Showing ${visible.length} of ${searchUsers.length} users`;
        searchCount.hidden = false;
    } else {
        searchCount.textContent = "";
        searchCount.hidden = true;
    }
}

function visibleSearchUsers() {
    const visible = searchUsers.filter(matchesSearchFilters);
    const direction = searchDirection.value === "desc" ? -1 : 1;
    const sort = searchSort.value;

    visible.sort((left, right) => {
        const primary = compareSearchField(left, right, sort);
        if (primary !== 0) {
            return primary * direction;
        }
        const byName = compareSearchNames(left, right);
        if (byName !== 0) {
            return byName;
        }
        return String(left.steamId).localeCompare(String(right.steamId));
    });

    return visible;
}

function compareSearchField(left, right, field) {
    switch (field) {
        case "updated":
            return compareSearchDates(left, right);
        case "owned":
            return (
                countValue(left.ownedAppCount) - countValue(right.ownedAppCount)
            );
        case "wishlist":
            return (
                countValue(left.wishlistAppCount) -
                countValue(right.wishlistAppCount)
            );
        case "name":
        default:
            return compareSearchNames(left, right);
    }
}

function compareSearchNames(left, right) {
    return String(left.name ?? "").localeCompare(
        String(right.name ?? ""),
        undefined,
        {
            sensitivity: "base",
        },
    );
}

function compareSearchDates(left, right) {
    const leftTime = Date.parse(left.updatedAt ?? "");
    const rightTime = Date.parse(right.updatedAt ?? "");
    return (
        (Number.isNaN(leftTime) ? 0 : leftTime) -
        (Number.isNaN(rightTime) ? 0 : rightTime)
    );
}

function countValue(value) {
    const number = Number(value);
    return Number.isFinite(number) ? number : 0;
}

function matchesSearchFilters(user) {
    const wishlist = searchWishlist.value;
    if (wishlist === "public" && user.wishlistPrivate) {
        return false;
    }
    if (wishlist === "private" && !user.wishlistPrivate) {
        return false;
    }

    return true;
}

// Map the latest import job status onto the status shown in the UI.
function importStatus(latestStatus) {
    switch (latestStatus) {
        case "queued":
            return { label: "Pending", tone: "queued" };
        case "running":
            return { label: "Processing", tone: "running" };
        case "succeeded":
            return { label: "Available", tone: "succeeded" };
        default:
            return { label: "Out-of-Date", tone: "out-of-date" };
    }
}

function resetSearchControls() {
    searchSort.value = "name";
    searchDirection.value = SEARCH_SORT_DEFAULT_DIRECTIONS.name;
    searchWishlist.value = "all";
}

function buildSearchResultRows(user, index) {
    const row = document.createElement("tr");
    row.className = "app-row";

    const nameCell = newElement("td", "app-cell");
    const body = newElement("div", "app-cell__body");
    const avatar = user.avatarMedium || user.avatarFull || user.avatar;
    if (avatar) {
        const image = newElement(
            "img",
            "app-cell__image app-cell__image--avatar",
        );
        image.src = avatar;
        image.alt = "";
        image.loading = "lazy";
        body.append(image);
    }

    const info = newElement("div", "app-cell__info");
    const nameLink = newElement("a", "app-cell__name", user.name ?? "");
    nameLink.href = `/user?name=${encodeURIComponent(user.name ?? "")}`;
    nameLink.dataset.route = "/user";
    info.append(nameLink);

    // Match View User's Wishlist privacy line: the privacy state is a badge,
    // not plain text, so Public stays green and Private stays red.
    const counts = newElement("p", "app-cell__meta");
    counts.append(
        document.createTextNode(
            `Owned: ${formatCount(user.ownedAppCount)} / Wishlist: ${formatCount(
                user.wishlistAppCount,
            )} `,
        ),
        badge(
            user.wishlistPrivate ? "Private" : "Public",
            user.wishlistPrivate ? "private" : "public",
        ),
    );
    info.append(counts);

    body.append(info);
    nameCell.append(body);

    const status = importStatus(user.latestImportStatus ?? null);
    const statusCell = newElement("td");
    statusCell.append(badge(status.label, status.tone));

    const detailsID = `search-user-details-${user.steamId ?? "unknown"}-${index}`;
    const toggle = newElement("button", "button--secondary app-expand");
    toggle.type = "button";
    toggle.setAttribute("aria-expanded", "false");
    toggle.setAttribute("aria-controls", detailsID);
    toggle.setAttribute(
        "aria-label",
        `Show details for ${user.name ?? "user"}`,
    );

    const chevron = newElement("span", "app-expand__chevron");
    chevron.setAttribute("aria-hidden", "true");
    toggle.append(chevron);

    const expandCell = newElement("td", "app-expand-cell");
    expandCell.append(toggle);

    const detailsRow = document.createElement("tr");
    detailsRow.className = "app-details-row";
    detailsRow.hidden = true;

    const detailsCell = newElement("td", "app-details-cell");
    detailsCell.colSpan = 3;

    const details = newElement("div", "app-details");
    details.id = detailsID;
    details.append(buildSearchResultDetails(user));
    detailsCell.append(details);
    detailsRow.append(detailsCell);

    toggle.addEventListener("click", () => {
        const expanded = toggle.getAttribute("aria-expanded") === "true";
        toggle.setAttribute("aria-expanded", String(!expanded));
        toggle.setAttribute(
            "aria-label",
            expanded
                ? `Show details for ${user.name ?? "user"}`
                : `Hide details for ${user.name ?? "user"}`,
        );
        detailsRow.hidden = expanded;
    });

    row.append(nameCell, statusCell, expandCell);

    return [row, detailsRow];
}

function buildSearchResultDetails(user) {
    const details = newElement("dl", "details app-details__list");
    // Steam IDs exceed JavaScript's safe integer range, so keep them as text.
    details.append(
        detailRow("Steam user name", user.name ?? ""),
        detailRow("Steam ID", String(user.steamId ?? "")),
        detailRow("Personaname", user.personaName || "—"),
        buildProfileURLRow(user.profileUrl),
    );
    return details;
}

function buildProfileURLRow(profileUrl) {
    const href = safeExternalURL(profileUrl);
    if (!href) {
        return detailRow("Profile URL", "—");
    }

    const link = newElement("a", "app-details__website", href);
    link.href = href;
    link.target = "_blank";
    link.rel = "noopener noreferrer";
    return detailRowNode("Profile URL", link);
}

function searchStat(label, value) {
    const row = newElement("div", "result__stat");
    row.append(newElement("dt", "", label), newElement("dd", "", value));
    return row;
}

function badge(text, status) {
    const node = newElement("span", "badge", text);
    node.dataset.status = status;
    return node;
}

function newElement(tag, className, text) {
    const element = document.createElement(tag);
    if (className) {
        element.className = className;
    }
    if (text !== undefined) {
        element.textContent = text;
    }
    return element;
}

function setSearchStatus(message, state) {
    searchStatus.textContent = message;
    if (state) {
        searchStatus.dataset.state = state;
    } else {
        delete searchStatus.dataset.state;
    }
}

function setSearchBusy(busy) {
    searchBusy = busy;
    searchInput.disabled = busy;
    searchButton.disabled = busy;
}

function isSearchCurrent(token) {
    return token === searchToken;
}

function leaveSearch() {
    searchController?.abort();
    if (searchBusy) {
        setSearchBusy(false);
        setSearchStatus("");
    }
}

function enterUser() {
    const parsed = parseUserQuery(location.search);

    userState.name = parsed.name;
    userState.list = parsed.list;
    userState.page = parsed.page;
    userState.pageSize = parsed.pageSize;
    userState.pages = { owned: 1, wishlist: 1 };
    userState.pages[parsed.list] = parsed.page;
    userState.data = null;

    userController?.abort();
    userController = null;
    userToken += 1;

    userInput.value = parsed.name;
    userPageSize.value = String(parsed.pageSize);
    setActiveTab(parsed.list);
    setUserLoading(false);
    clearUserView();

    if (!parsed.name) {
        setUserStatus("");
        return;
    }

    syncUserURL();
    void loadUser();
}

function leaveUser() {
    userController?.abort();
    userController = null;
    userToken += 1;
}

function parseUserQuery(search) {
    const params = new URLSearchParams(search);
    const name = (params.get("name") ?? "").trim();
    const list = params.get("list") === "wishlist" ? "wishlist" : "owned";
    const page = parsePositiveInteger(params.get("page"), 1);
    const pageSize = userPageSizeFromValue(params.get("pageSize"));

    return { name, list, page, pageSize };
}

function parsePositiveInteger(value, fallback) {
    const number = Number(value);
    return Number.isInteger(number) && number >= 1 ? number : fallback;
}

function userPageSizeFromValue(value) {
    const number = Number(value);
    return APP_PAGE_SIZES.includes(number) ? number : 25;
}

function buildUserURL(name, list, page, pageSize) {
    const query = [
        `name=${encodeURIComponent(name)}`,
        `list=${encodeURIComponent(list)}`,
        `page=${page}`,
        `pageSize=${pageSize}`,
    ];
    return `/user?${query.join("&")}`;
}

function pushUserURL(name, list, page, pageSize) {
    const url = buildUserURL(name, list, page, pageSize);
    if (url !== location.pathname + location.search + location.hash) {
        history.pushState({}, "", url);
        currentHref = url;
    }

    userState.name = name;
    userState.list = list;
    userState.page = page;
    userState.pageSize = pageSize;
    userState.pages[list] = page;

    userInput.value = name;
    userPageSize.value = String(pageSize);
    setActiveTab(list);
    void loadUser();
}

function syncUserURL() {
    if (!userState.name) {
        return;
    }

    const url = buildUserURL(
        userState.name,
        userState.list,
        userState.page,
        userState.pageSize,
    );
    if (url !== location.pathname + location.search + location.hash) {
        history.replaceState({}, "", url);
        currentHref = url;
    }
}

function selectUserList(list) {
    if (!userState.name) {
        setActiveTab(list);
        return;
    }
    if (userState.list === list) {
        return;
    }

    const page = userState.pages[list] ?? 1;
    userState.pages[list] = page;
    pushUserURL(userState.name, list, page, userState.pageSize);
}

function goToUserPage(page) {
    if (!userState.name || page === userState.page) {
        return;
    }

    const totalPages = countValue(userState.data?.totalPages);
    if (!Number.isInteger(page) || page < 1 || page > totalPages) {
        return;
    }

    userState.pages[userState.list] = page;
    pushUserURL(userState.name, userState.list, page, userState.pageSize);
}

async function loadUser() {
    const token = ++userToken;
    userController?.abort();
    const controller = new AbortController();
    userController = controller;

    if (!userState.name) {
        setUserLoading(false);
        clearUserView();
        setUserStatus("");
        userController = null;
        return;
    }

    setUserLoading(true);
    setUserStatus(`Loading “${userState.name}”…`);

    const params = new URLSearchParams({
        name: userState.name,
        list: userState.list,
        page: String(userState.page),
        pageSize: String(userState.pageSize),
    });

    try {
        const response = await apiFetch(
            `/api/steamusers/by-name?${params.toString()}`,
            {
                headers: { Accept: "application/json" },
                signal: controller.signal,
            },
        );
        if (!isUserCurrent(token)) {
            return;
        }

        if (response.status === 404) {
            clearUserView();
            setUserLoading(false);
            showUserNotFound();
            return;
        }
        if (!response.ok) {
            throw await readError(response);
        }

        const body = await response.json();
        if (!isUserCurrent(token)) {
            return;
        }

        applyUserResponse(body);
        setUserStatus("");
    } catch (error) {
        if (isAbort(error) || !isUserCurrent(token)) {
            return;
        }
        setUserStatus(errorMessage(error), "error");
    } finally {
        if (isUserCurrent(token)) {
            setUserLoading(false);
            userController = null;
        }
    }
}

function applyUserResponse(body) {
    const user = body?.steamUser;
    if (!user) {
        throw new Error("The server did not return a Steam user.");
    }

    userState.data = body;

    if (typeof user.name === "string" && user.name !== "") {
        userState.name = user.name;
        userInput.value = user.name;
    }
    userState.list = body.list === "wishlist" ? "wishlist" : "owned";
    userState.page = parsePositiveInteger(body.page, 1);
    userState.pageSize = userPageSizeFromValue(body.pageSize);
    userState.pages[userState.list] = userState.page;

    userPageSize.value = String(userState.pageSize);
    setActiveTab(userState.list);

    renderUserSummary(body);
    renderUserApps(body);
    syncUserURL();
}

function renderUserSummary(data) {
    const user = data.steamUser;
    const latest = data.latestImport ?? null;

    userSummary.hidden = false;
    const status = importStatus(latest?.status ?? null);
    userStatusBadge.textContent = status.label;
    userStatusBadge.dataset.status = status.tone;

    const rows = [];

    // Show the Steam avatar above the name, the same way Search Users shows
    // it in each result row. Users without profile data have no avatar.
    const avatar = user.avatarMedium || user.avatarFull || user.avatar;
    if (avatar) {
        const image = newElement("img", "user-summary__avatar");
        image.src = avatar;
        image.alt = "";
        image.loading = "lazy";
        const avatarValue = newElement("dd");
        avatarValue.append(image);
        const avatarRow = newElement("div", "details__row details__row--avatar");
        avatarRow.append(newElement("dt", "visually-hidden", "Avatar"), avatarValue);
        rows.push(avatarRow);
    }

    rows.push(
        detailRow("Steam name", user.name ?? ""),
        // Steam IDs exceed JavaScript's safe integer range, so keep them as text.
        detailRow(
            "Steam ID",
            user.steamId === null || user.steamId === undefined
                ? ""
                : String(user.steamId),
        ),
        detailRow("Personaname", user.personaName || "—"),
        buildProfileURLRow(user.profileUrl),
        detailRowNode(
            "Wishlist privacy",
            badge(
                user.wishlistPrivate ? "Private" : "Public",
                user.wishlistPrivate ? "private" : "public",
            ),
        ),
        detailRow("Owned apps", formatCount(data.ownedAppCount ?? 0)),
        detailRow("Wishlist apps", formatCount(data.wishlistAppCount ?? 0)),
    );

    if (latest?.error) {
        rows.push(detailRow("Import error", latest.error));
    }

    userSummaryDetails.replaceChildren(...rows);

    if (user.wishlistPrivate) {
        userSummaryNotice.textContent =
            "This user's wishlist is private; imported wishlist data may be incomplete.";
        userSummaryNotice.hidden = false;
    } else {
        userSummaryNotice.textContent = "";
        userSummaryNotice.hidden = true;
    }
}

function renderUserApps(data) {
    const list = data.list === "wishlist" ? "wishlist" : "owned";
    const apps = Array.isArray(data.apps) ? data.apps : [];
    const total = countValue(data.total);
    const page = parsePositiveInteger(data.page, 1);
    const totalPages = countValue(data.totalPages);

    userApps.hidden = false;
    ownedTabCount.textContent = formatCount(data.ownedAppCount ?? 0);
    wishlistTabCount.textContent = formatCount(data.wishlistAppCount ?? 0);

    if (total === 0) {
        userListCount.textContent =
            list === "wishlist" ? "No wishlist apps." : "No owned apps.";
    } else {
        const noun = total === 1 ? "app" : "apps";
        userListCount.textContent = `Page ${page} of ${totalPages} · ${formatCount(
            total,
        )} ${list} ${noun}`;
    }

    userAppRows.replaceChildren(
        ...apps.flatMap((app, index) => buildUserAppRows(app, index)),
    );
    renderUserPagination(data);
}

function buildUserAppRows(app, index) {
    const row = document.createElement("tr");
    row.className = "app-row";

    const nameCell = newElement("td", "app-cell");
    const body = newElement("div", "app-cell__body");
    if (app.headerImage) {
        const image = newElement("img", "app-cell__image");
        image.src = app.headerImage;
        image.alt = "";
        image.loading = "lazy";
        body.append(image);
    }
    const info = newElement("div", "app-cell__info");
    info.append(
        newElement("span", "app-cell__name", app.name ?? "Unknown app"),
    );

    const releaseLabel = appReleaseLabel(app);
    if (releaseLabel !== "—") {
        info.append(newElement("p", "app-cell__meta", releaseLabel));
    }

    body.append(info);
    nameCell.append(body);

    const storeCell = newElement("td", "app-store");

    const priceLabel = appPriceLabel(app);
    const storeLink = newElement("a", "app-store__link");
    storeLink.append(buildAppPriceContent(app));
    storeLink.href = `https://store.steampowered.com/app/${encodeURIComponent(app.appId)}/`;
    storeLink.target = "_blank";
    storeLink.rel = "noopener noreferrer";
    storeLink.setAttribute(
        "aria-label",
        `View ${app.name ?? "app"} on Steam (${priceLabel})`,
    );
    storeCell.append(storeLink);

    const detailsID = `user-app-details-${app.appId ?? "unknown"}-${index}`;
    const toggle = newElement("button", "button--secondary app-expand");
    toggle.type = "button";
    toggle.setAttribute("aria-expanded", "false");
    toggle.setAttribute("aria-controls", detailsID);
    toggle.setAttribute("aria-label", `Show details for ${app.name ?? "app"}`);

    const chevron = newElement("span", "app-expand__chevron");
    chevron.setAttribute("aria-hidden", "true");
    toggle.append(chevron);

    const expandCell = newElement("td", "app-expand-cell");
    expandCell.append(toggle);

    const detailsRow = document.createElement("tr");
    detailsRow.className = "app-details-row";
    detailsRow.hidden = true;

    const detailsCell = newElement("td", "app-details-cell");
    detailsCell.colSpan = 3;

    const details = newElement("div", "app-details");
    details.id = detailsID;
    details.append(buildUserAppDetails(app));
    detailsCell.append(details);
    detailsRow.append(detailsCell);

    toggle.addEventListener("click", () => {
        const expanded = toggle.getAttribute("aria-expanded") === "true";
        toggle.setAttribute("aria-expanded", String(!expanded));
        toggle.setAttribute(
            "aria-label",
            expanded
                ? `Show details for ${app.name ?? "app"}`
                : `Hide details for ${app.name ?? "app"}`,
        );
        detailsRow.hidden = expanded;
    });

    row.append(nameCell, storeCell, expandCell);

    return [row, detailsRow];
}

function appPriceLabel(app) {
    const formatted =
        typeof app.priceFinalFormatted === "string"
            ? app.priceFinalFormatted.trim()
            : "";
    const discount = countValue(app.priceDiscountPercent);
    const initial = formatAppPrice(app.priceInitial, app.priceCurrency);

    if (formatted && discount > 0) {
        return initial
            ? `${formatted}, ${discount}% off from ${initial}`
            : `${formatted}, ${discount}% off`;
    }
    return formatted || "View on Steam";
}

function buildAppPriceContent(app) {
    const formatted =
        typeof app.priceFinalFormatted === "string"
            ? app.priceFinalFormatted.trim()
            : "";
    const discount = countValue(app.priceDiscountPercent);
    const initial = formatAppPrice(app.priceInitial, app.priceCurrency);

    if (discount <= 0 || (!formatted && !initial)) {
        return document.createTextNode(formatted || "View on Steam");
    }

    const price = newElement("span", "app-store__price");
    price.append(newElement("span", "app-store__discount", `-${discount}%`));

    const amounts = newElement("span", "app-store__amounts");
    if (initial) {
        amounts.append(newElement("s", "app-store__original", initial));
    }
    if (formatted) {
        amounts.append(newElement("span", "app-store__final", formatted));
    }
    price.append(amounts);

    return price;
}

function buildUserAppDetails(app) {
    const fragment = document.createDocumentFragment();

    if (app.headerImage) {
        const image = newElement(
            "img",
            "app-cell__image app-cell__image--large",
        );
        image.src = app.headerImage;
        image.alt = "";
        image.loading = "lazy";
        fragment.append(image);
    }

    if (
        typeof app.shortDescription === "string" &&
        app.shortDescription.trim() !== ""
    ) {
        fragment.append(
            newElement(
                "p",
                "app-details__description",
                app.shortDescription.trim(),
            ),
        );
    }

    const rows = newElement("dl", "details app-details__list");
    rows.append(
        detailRow("Release date", appReleaseLabel(app)),
        detailRow("Price", appPriceDetail(app)),
        detailRow("Platforms", appPlatformsLabel(app)),
        buildAppWebsiteRow(app.website),
    );
    fragment.append(rows);

    return fragment;
}

function appReleaseLabel(app) {
    const releaseDate =
        typeof app.releaseDate === "string" ? app.releaseDate.trim() : "";
    if (releaseDate) {
        return app.releaseComingSoon === true
            ? `${releaseDate} (coming soon)`
            : releaseDate;
    }
    return app.releaseComingSoon === true ? "Coming soon" : "—";
}

function appPriceDetail(app) {
    const formatted =
        typeof app.priceFinalFormatted === "string"
            ? app.priceFinalFormatted.trim()
            : "";
    const discount = countValue(app.priceDiscountPercent);
    const initial = formatAppPrice(app.priceInitial, app.priceCurrency);

    if (formatted && discount > 0) {
        return initial
            ? `${formatted} (${discount}% off from ${initial})`
            : `${formatted} (${discount}% off)`;
    }
    return formatted || "—";
}

function formatAppPrice(cents, currency) {
    if (
        !Number.isInteger(cents) ||
        typeof currency !== "string" ||
        !/^[A-Za-z]{3}$/.test(currency)
    ) {
        return "";
    }
    try {
        return new Intl.NumberFormat(undefined, {
            style: "currency",
            currency: currency.toUpperCase(),
        }).format(cents / 100);
    } catch {
        return "";
    }
}

function appPlatformsLabel(app) {
    const platforms = [];
    if (app.platformWindows === true) {
        platforms.push("Windows");
    }
    if (app.platformMac === true) {
        platforms.push("macOS");
    }
    if (app.platformLinux === true) {
        platforms.push("Linux");
    }
    return platforms.length > 0 ? platforms.join(", ") : "—";
}

function buildAppWebsiteRow(website) {
    const href = safeExternalURL(website);
    if (!href) {
        return detailRow("Website", "—");
    }

    const link = newElement("a", "app-details__website", href);
    link.href = href;
    link.target = "_blank";
    link.rel = "noopener noreferrer";
    return detailRowNode("Website", link);
}

function safeExternalURL(value) {
    if (typeof value !== "string" || value.trim() === "") {
        return "";
    }
    try {
        const url = new URL(value.trim());
        if (url.protocol !== "http:" && url.protocol !== "https:") {
            return "";
        }
        return url.href;
    } catch {
        return "";
    }
}

function renderUserPagination(data) {
    userPagination.replaceChildren();

    const current = parsePositiveInteger(data.page, 1);
    const totalPages = countValue(data.totalPages);
    const hasPages = totalPages > 0;

    userPagination.append(
        paginationButton(
            "Previous",
            current - 1,
            current <= 1 || !hasPages,
            "Previous page",
        ),
    );

    if (hasPages) {
        for (const item of paginationItems(current, totalPages)) {
            if (item === "ellipsis") {
                const ellipsis = newElement(
                    "span",
                    "pagination__ellipsis",
                    "…",
                );
                ellipsis.setAttribute("aria-hidden", "true");
                userPagination.append(ellipsis);
                continue;
            }

            const button = paginationButton(
                String(item),
                item,
                false,
                `Page ${item}`,
            );
            if (item === current) {
                button.setAttribute("aria-current", "page");
                button.disabled = true;
            }
            userPagination.append(button);
        }
    }

    userPagination.append(
        paginationButton(
            "Next",
            current + 1,
            !hasPages || current >= totalPages,
            "Next page",
        ),
    );
}

function paginationButton(text, page, disabled, label) {
    const button = newElement("button", "pagination__button", text);
    button.type = "button";
    button.disabled = disabled;
    button.setAttribute("aria-label", label);
    button.addEventListener("click", () => goToUserPage(page));
    return button;
}

function paginationItems(current, totalPages) {
    if (totalPages <= 7) {
        return Array.from({ length: totalPages }, (_, index) => index + 1);
    }

    const items = [1];
    const start = Math.max(2, current - 1);
    const end = Math.min(totalPages - 1, current + 1);

    if (start > 2) {
        items.push("ellipsis");
    }
    for (let page = start; page <= end; page += 1) {
        items.push(page);
    }
    if (end < totalPages - 1) {
        items.push("ellipsis");
    }
    items.push(totalPages);

    return items;
}

function setActiveTab(list) {
    const wishlist = list === "wishlist";
    tabOwned.setAttribute("aria-selected", String(!wishlist));
    tabWishlist.setAttribute("aria-selected", String(wishlist));
    userAppPanel.setAttribute(
        "aria-labelledby",
        wishlist ? "tab-wishlist" : "tab-owned",
    );
}

function clearUserView() {
    userState.data = null;
    userSummary.hidden = true;
    userApps.hidden = true;
    userSummaryDetails.replaceChildren();
    userSummaryNotice.textContent = "";
    userSummaryNotice.hidden = true;
    ownedTabCount.textContent = "0";
    wishlistTabCount.textContent = "0";
    userListCount.textContent = "";
    userAppRows.replaceChildren();
    userPagination.replaceChildren();
}

function setUserLoading(loading) {
    userSummary.setAttribute("aria-busy", String(loading));
    userApps.setAttribute("aria-busy", String(loading));
}

function setUserStatus(message, state) {
    userStatus.replaceChildren();
    userStatus.textContent = message;
    if (state) {
        userStatus.dataset.state = state;
    } else {
        delete userStatus.dataset.state;
    }
}

function showUserNotFound() {
    userStatus.replaceChildren();
    userStatus.dataset.state = "error";
    userStatus.append(document.createTextNode("Steam user not found. "));

    const link = newElement("a", "", "Search Users");
    link.href = "/search";
    link.dataset.route = "/search";
    userStatus.append(link);
}

function isUserCurrent(token) {
    return token === userToken;
}

function detailRow(label, value) {
    const row = newElement("div", "details__row");
    row.append(newElement("dt", "", label), newElement("dd", "", value));
    return row;
}

function detailRowNode(label, node) {
    const row = newElement("div", "details__row");
    const value = newElement("dd");
    value.append(node);
    row.append(newElement("dt", "", label), value);
    return row;
}

function enterCommonGames() {
    const parsed = parseCommonGamesQuery(location.search);
    const validUserCount =
        parsed.steamIDs.length >= 2 &&
        parsed.steamIDs.length <= MAX_COMMON_GAME_USERS;

    commonGamesState.selectedUsers.clear();
    commonGamesState.searchUsers = [];
    commonGamesState.results = null;
    commonGamesState.submitted = parsed.submitted && validUserCount;
    commonGamesLimit.value = String(parsed.limit ?? COMMON_GAMES_LIMIT_DEFAULT);

    clearCommonGamesSearchResults();
    clearCommonGamesResultsView();
    renderCommonGamesSelectedUsers();
    setCommonGamesStatus("");

    if (parsed.steamIDs.length === 0) {
        return;
    }
    if (!validUserCount) {
        setCommonGamesStatus(
            `Shared links must include between 2 and ${MAX_COMMON_GAME_USERS} users.`,
            "error",
        );
        return;
    }

    void loadCommonGamesUsersFromURL(
        parsed.steamIDs,
        commonGamesState.submitted,
    );
}

function leaveCommonGames() {
    commonGamesSearchController?.abort();
    commonGamesSearchController = null;
    commonGamesSearchToken += 1;
    if (commonGamesSearchBusy) {
        setCommonGamesSearchBusy(false);
        setCommonGamesSearchStatus("");
    }

    commonGamesUsersController?.abort();
    commonGamesUsersController = null;
    commonGamesUsersToken += 1;

    commonGamesRequestController?.abort();
    commonGamesRequestController = null;
    commonGamesRequestToken += 1;
    if (commonGamesBusy) {
        commonGamesBusy = false;
        setCommonGamesStatus("");
    }

    commonGamesResults.setAttribute("aria-busy", "false");
    syncCommonGamesControls();
}

function parseCommonGamesQuery(search) {
    const params = new URLSearchParams(search);
    const seen = new Set();
    const steamIDs = [];

    for (const rawID of (params.get("steamIds") ?? "").split(",")) {
        const value = rawID.trim();
        if (!/^\d+$/.test(value)) {
            continue;
        }
        const steamID = value.replace(/^0+(?=\d)/, "");
        if (steamID === "0" || seen.has(steamID)) {
            continue;
        }
        seen.add(steamID);
        steamIDs.push(steamID);
    }

    return {
        steamIDs,
        limit: commonGamesLimitFromValue(params.get("limit")),
        submitted: params.get("submitted") === "1",
    };
}

function buildCommonGamesURL() {
    const params = new URLSearchParams();
    const steamIDs = [...commonGamesState.selectedUsers.keys()];
    if (steamIDs.length > 0) {
        params.set("steamIds", steamIDs.join(","));
    }

    const limit = parseCommonGamesLimit();
    if (limit !== null) {
        params.set("limit", String(limit));
    }
    if (commonGamesState.submitted) {
        params.set("submitted", "1");
    }

    const query = params.toString();
    return query ? `/common-games?${query}` : "/common-games";
}

function syncCommonGamesURL(push = false) {
    const url = buildCommonGamesURL();
    if (url === location.pathname + location.search + location.hash) {
        return;
    }

    if (push) {
        history.pushState({}, "", url);
    } else {
        history.replaceState({}, "", url);
    }
    currentHref = url;
}

async function loadCommonGamesUsersFromURL(steamIDs, submitted) {
    const token = ++commonGamesUsersToken;
    commonGamesUsersController?.abort();
    const controller = new AbortController();
    commonGamesUsersController = controller;

    setCommonGamesStatus(
        `Loading ${formatCount(steamIDs.length)} shared ${
            steamIDs.length === 1 ? "user" : "users"
        }…`,
    );

    try {
        const response = await apiFetch(
            `/api/steamusers/by-ids?steamIds=${encodeURIComponent(
                steamIDs.join(","),
            )}`,
            {
                headers: { Accept: "application/json" },
                signal: controller.signal,
            },
        );
        if (!isCommonGamesUsersCurrent(token)) {
            return;
        }
        if (!response.ok) {
            throw await readError(response);
        }

        const body = await response.json();
        if (!isCommonGamesUsersCurrent(token)) {
            return;
        }

        const users = Array.isArray(body?.steamUsers) ? body.steamUsers : [];
        const usersByID = new Map(
            users.map((user) => [String(user.steamId ?? ""), user]),
        );

        commonGamesState.selectedUsers.clear();
        for (const steamID of steamIDs) {
            const user = usersByID.get(steamID);
            if (user?.hasSucceededImport === true) {
                commonGamesState.selectedUsers.set(steamID, user);
            }
        }
        renderCommonGamesSelectedUsers();

        if (commonGamesState.selectedUsers.size < 2) {
            setCommonGamesStatus(
                "This shared link no longer includes enough imported users.",
                "error",
            );
            return;
        }

        const missingCount =
            steamIDs.length - commonGamesState.selectedUsers.size;
        if (missingCount > 0) {
            setCommonGamesStatus(
                `${formatCount(missingCount)} shared ${
                    missingCount === 1 ? "user is" : "users are"
                } no longer available. Select at least 2 users and try again.`,
                "error",
            );
            return;
        }

        setCommonGamesStatus("");
        if (submitted) {
            await findCommonGames();
        }
    } catch (error) {
        if (isAbort(error) || !isCommonGamesUsersCurrent(token)) {
            return;
        }
        setCommonGamesStatus(errorMessage(error), "error");
    } finally {
        if (isCommonGamesUsersCurrent(token)) {
            commonGamesUsersController = null;
        }
    }
}

function isCommonGamesUsersCurrent(token) {
    return token === commonGamesUsersToken;
}

async function performCommonGamesSearch() {
    const token = ++commonGamesSearchToken;
    commonGamesSearchController?.abort();
    commonGamesSearchController = null;

    const name = commonGamesSearchInput.value.trim();
    if (!name) {
        clearCommonGamesSearchResults();
        setCommonGamesSearchStatus("Enter a Steam name.", "error");
        commonGamesSearchInput.focus();
        return;
    }

    const controller = new AbortController();
    commonGamesSearchController = controller;

    setCommonGamesSearchBusy(true);
    setCommonGamesSearchStatus(`Searching for “${name}”…`);

    try {
        const response = await apiFetch(
            `/api/steamusers/search?name=${encodeURIComponent(name)}`,
            {
                headers: { Accept: "application/json" },
                signal: controller.signal,
            },
        );
        if (!isCommonGamesSearchCurrent(token)) {
            return;
        }
        if (!response.ok) {
            throw await readError(response);
        }

        const body = await response.json();
        if (!isCommonGamesSearchCurrent(token)) {
            return;
        }

        commonGamesState.searchUsers = Array.isArray(body?.steamUsers)
            ? body.steamUsers
            : [];

        if (commonGamesState.searchUsers.length === 0) {
            clearCommonGamesSearchResults();
            setCommonGamesSearchStatus(`No users match “${name}”.`);
            return;
        }

        renderCommonGamesSearchResults();
        setCommonGamesSearchStatus("");
    } catch (error) {
        if (isAbort(error) || !isCommonGamesSearchCurrent(token)) {
            return;
        }
        clearCommonGamesSearchResults();
        setCommonGamesSearchStatus(errorMessage(error), "error");
    } finally {
        if (isCommonGamesSearchCurrent(token)) {
            setCommonGamesSearchBusy(false);
            commonGamesSearchController = null;
        }
    }
}

function renderCommonGamesSearchResults() {
    if (commonGamesState.searchUsers.length === 0) {
        commonGamesSearchResults.replaceChildren();
        commonGamesSearchResultsPanel.hidden = true;
        return;
    }

    commonGamesSearchResultsPanel.hidden = false;
    commonGamesSearchResults.replaceChildren(
        ...commonGamesState.searchUsers.map(buildCommonGamesSearchResult),
    );
}

function clearCommonGamesSearchResults() {
    commonGamesState.searchUsers = [];
    commonGamesSearchResults.replaceChildren();
    commonGamesSearchResultsPanel.hidden = true;
}

function commonGamesUserStatus(user) {
    if (user.hasSucceededImport === true) {
        return { label: "Ready", tone: "succeeded" };
    }
    if (user.latestImportStatus) {
        return importStatus(user.latestImportStatus);
    }
    return { label: "Never imported", tone: "out-of-date" };
}

function buildCommonGamesSearchResult(user) {
    const item = newElement("li", "result");
    const main = newElement("div", "result__main");
    const status = commonGamesUserStatus(user);

    const heading = newElement("div", "result__heading");
    heading.append(
        newElement("p", "result__name", user.name ?? ""),
        badge(status.label, status.tone),
    );
    main.append(heading);
    main.append(
        newElement("p", "result__id", `Steam ID ${user.steamId ?? ""}`),
    );

    const stats = newElement("dl", "result__stats");
    stats.append(
        searchStat("Owned apps", formatCount(user.ownedAppCount)),
        searchStat("Wishlist apps", formatCount(user.wishlistAppCount)),
    );
    main.append(stats);

    const steamID = String(user.steamId ?? "");
    const selected = commonGamesState.selectedUsers.has(steamID);
    const atLimit =
        commonGamesState.selectedUsers.size >= MAX_COMMON_GAME_USERS;
    const available = user.hasSucceededImport === true;

    const action = newElement("button", "result__link");
    action.type = "button";
    action.dataset.addSteamId = steamID;

    if (selected) {
        action.textContent = "Added";
        action.disabled = true;
    } else if (!available) {
        action.textContent = "Unavailable";
        action.disabled = true;
        action.title = "No completed import";
    } else if (atLimit) {
        action.textContent = "Add";
        action.disabled = true;
        action.title = `Selection limit reached (${MAX_COMMON_GAME_USERS} users).`;
    } else {
        action.textContent = "Add";
    }

    item.append(main, action);

    if (!available) {
        main.append(newElement("p", "hint", "No completed import yet."));
    }

    return item;
}

function addCommonGamesUser(user) {
    const steamID = String(user.steamId ?? "");
    if (
        steamID === "" ||
        user.hasSucceededImport !== true ||
        commonGamesState.selectedUsers.size >= MAX_COMMON_GAME_USERS ||
        commonGamesState.selectedUsers.has(steamID)
    ) {
        return;
    }

    commonGamesState.selectedUsers.set(steamID, user);
    invalidateCommonGamesResults();
    renderCommonGamesSelectedUsers();
    renderCommonGamesSearchResults();
}

function removeCommonGamesUser(steamID) {
    if (!commonGamesState.selectedUsers.delete(String(steamID))) {
        return;
    }

    invalidateCommonGamesResults();
    renderCommonGamesSelectedUsers();
    renderCommonGamesSearchResults();
}

function renderCommonGamesSelectedUsers() {
    const users = [...commonGamesState.selectedUsers.values()];
    const atLimit = users.length >= MAX_COMMON_GAME_USERS;

    commonGamesSelectedCount.textContent = `${users.length} / ${MAX_COMMON_GAME_USERS}`;
    commonGamesSelectedEmpty.hidden = users.length > 0;
    commonGamesSelectedHint.hidden = !atLimit;
    commonGamesSelectedHint.textContent = atLimit
        ? `Selection limit reached (${MAX_COMMON_GAME_USERS} users).`
        : "";
    commonGamesSelectedList.replaceChildren(
        ...users.map(buildCommonGamesSelectedUser),
    );

    syncCommonGamesControls();
}

function buildCommonGamesSelectedUser(user) {
    const item = newElement("li", "selected-user");
    const main = newElement("div", "selected-user__main");
    main.append(
        newElement("span", "selected-user__name", user.name ?? ""),
        newElement("span", "selected-user__id", String(user.steamId ?? "")),
    );

    const remove = newElement(
        "button",
        "button--secondary selected-user__remove",
        "Remove",
    );
    remove.type = "button";
    remove.dataset.removeSteamId = String(user.steamId ?? "");
    remove.setAttribute("aria-label", `Remove ${user.name ?? "user"}`);
    item.append(main, remove);

    return item;
}

function syncCommonGamesControls() {
    const enoughUsers = commonGamesState.selectedUsers.size >= 2;

    commonGamesSubmit.disabled = !enoughUsers || commonGamesBusy;
    commonGamesLimit.disabled = commonGamesBusy;
    commonGamesSubmitHint.hidden = enoughUsers;
    commonGamesSubmitHint.textContent = enoughUsers
        ? ""
        : "Select at least 2 users to continue.";
}

function parseCommonGamesLimit() {
    return commonGamesLimitFromValue(commonGamesLimit.value);
}

function commonGamesLimitFromValue(rawValue) {
    if (rawValue === null || rawValue.trim() === "") {
        return null;
    }

    const value = Number(rawValue);
    if (
        !Number.isInteger(value) ||
        value < COMMON_GAMES_LIMIT_MIN ||
        value > COMMON_GAMES_LIMIT_MAX
    ) {
        return null;
    }
    return value;
}

async function findCommonGames() {
    const users = [...commonGamesState.selectedUsers.values()];
    if (users.length < 2) {
        setCommonGamesStatus("Select at least 2 users.", "error");
        return;
    }

    const limit = parseCommonGamesLimit();
    if (limit === null) {
        setCommonGamesStatus("Enter a whole number from 1 to 100.", "error");
        commonGamesLimit.focus();
        return;
    }

    const alreadySubmitted = commonGamesState.submitted;
    commonGamesState.submitted = true;
    syncCommonGamesURL(!alreadySubmitted);

    const token = ++commonGamesRequestToken;
    commonGamesRequestController?.abort();
    const controller = new AbortController();
    commonGamesRequestController = controller;

    commonGamesBusy = true;
    syncCommonGamesControls();
    commonGamesState.results = null;
    clearCommonGamesResultsView();
    commonGamesResults.setAttribute("aria-busy", "true");
    setCommonGamesStatus("Finding common games…");

    try {
        const response = await apiFetch("/api/common-games", {
            method: "POST",
            headers: {
                Accept: "application/json",
                "Content-Type": "application/json",
            },
            body: JSON.stringify({
                steamIds: users.map((user) => String(user.steamId)),
                limit,
            }),
            signal: controller.signal,
        });
        if (!isCommonGamesRequestCurrent(token)) {
            return;
        }
        if (!response.ok) {
            throw await readError(response);
        }

        const body = await response.json();
        if (!isCommonGamesRequestCurrent(token)) {
            return;
        }

        const games = Array.isArray(body?.games) ? body.games : [];
        commonGamesState.results = { games, users, limit };
        renderCommonGamesResults();

        if (games.length === 0) {
            setCommonGamesStatus("");
        } else {
            setCommonGamesStatus(
                `Found ${formatCount(games.length)} ${
                    games.length === 1 ? "game" : "games"
                }.`,
            );
        }
    } catch (error) {
        if (isAbort(error) || !isCommonGamesRequestCurrent(token)) {
            return;
        }
        clearCommonGamesResultsView();
        setCommonGamesStatus(errorMessage(error), "error");
    } finally {
        if (isCommonGamesRequestCurrent(token)) {
            commonGamesBusy = false;
            commonGamesRequestController = null;
            commonGamesResults.setAttribute("aria-busy", "false");
            syncCommonGamesControls();
        }
    }
}

function renderCommonGamesResults() {
    const results = commonGamesState.results;
    if (!results) {
        return;
    }

    const { games, users } = results;
    commonGamesResults.hidden = false;
    commonGamesQueryUsers.replaceChildren(
        ...users.map(buildCommonGamesQueryUser),
    );
    commonGamesResultsCount.textContent = `${formatCount(games.length)} ${
        games.length === 1 ? "game" : "games"
    }`;
    commonGamesResultsEmpty.hidden = games.length !== 0;
    commonGamesTableScroll.hidden = games.length === 0;
    commonGamesNote.hidden = games.length === 0;
    commonGamesGameRows.replaceChildren(
        ...games.flatMap((game, index) =>
            buildCommonGameRows(game, users, index),
        ),
    );
}

function buildCommonGamesQueryUser(user) {
    const chip = newElement("span", "query-users__chip");
    const link = newElement("a", "query-users__name", user.name ?? "");
    link.href = `/user?name=${encodeURIComponent(user.name ?? "")}`;
    link.dataset.route = "/user";
    chip.append(link);
    return chip;
}

function buildCommonGameUsers(users) {
    if (users.length === 0) {
        return newElement("span", "details__empty", "None");
    }

    const container = newElement("span", "common-game-users");
    container.append(...users.map((user) => buildCommonGamesQueryUser(user)));
    return container;
}

function commonGamesUsersForIDs(users, steamIDs) {
    const ids = new Set(
        (Array.isArray(steamIDs) ? steamIDs : []).map((id) => String(id)),
    );
    return users.filter((user) => ids.has(String(user.steamId ?? "")));
}

function commonGamesMissingUsers(users, ownedSteamIDs, wishlistedSteamIDs) {
    const present = new Set(
        [
            ...(Array.isArray(ownedSteamIDs) ? ownedSteamIDs : []),
            ...(Array.isArray(wishlistedSteamIDs) ? wishlistedSteamIDs : []),
        ].map((id) => String(id)),
    );
    return users.filter((user) => !present.has(String(user.steamId ?? "")));
}

function detailRowWithUsers(label, value, users, accessibleLabel) {
    const summary = detailRow(label, value);
    summary.classList.add("details__row--with-users");

    const userRow = newElement("div", "details__row details__row--users");
    const term = newElement("dt", "visually-hidden", accessibleLabel);
    const description = newElement("dd", "details__users");
    description.append(buildCommonGameUsers(users));
    userRow.append(term, description);
    return [summary, userRow];
}

function buildCommonGameRows(game, users, index) {
    const row = document.createElement("tr");
    row.className = "app-row";

    const userCount = users.length;
    const nameCell = newElement("td", "app-cell");
    const body = newElement("div", "app-cell__body");
    if (game.headerImage) {
        const image = newElement("img", "app-cell__image");
        image.src = game.headerImage;
        image.alt = "";
        image.loading = "lazy";
        body.append(image);
    }

    const total = countValue(game.totalCommonUsers);
    const percent = commonGamesPercent(total, userCount);
    const meta = newElement("p", "app-cell__meta");
    meta.append(
        buildCommonGamesPercentBadge(percent),
        document.createTextNode(
            ` · ${formatCount(total)} of ${formatCount(
                userCount,
            )} users own or wishlist this game`,
        ),
    );

    const info = newElement("div", "app-cell__info");
    info.append(
        newElement("span", "app-cell__name", game.name ?? "Unknown app"),
        meta,
    );
    body.append(info);
    nameCell.append(body);

    const storeCell = newElement("td", "app-store");
    const priceLabel = appPriceLabel(game);
    const storeLink = newElement("a", "app-store__link");
    storeLink.append(buildAppPriceContent(game));
    storeLink.href = `https://store.steampowered.com/app/${encodeURIComponent(game.appId)}/`;
    storeLink.target = "_blank";
    storeLink.rel = "noopener noreferrer";
    storeLink.setAttribute(
        "aria-label",
        `View ${game.name ?? "app"} on Steam (${priceLabel})`,
    );
    storeCell.append(storeLink);

    const detailsID = `common-game-details-${game.appId ?? "unknown"}-${index}`;
    const toggle = newElement("button", "button--secondary app-expand");
    toggle.type = "button";
    toggle.setAttribute("aria-expanded", "false");
    toggle.setAttribute("aria-controls", detailsID);
    toggle.setAttribute("aria-label", `Show details for ${game.name ?? "app"}`);

    const chevron = newElement("span", "app-expand__chevron");
    chevron.setAttribute("aria-hidden", "true");
    toggle.append(chevron);

    const expandCell = newElement("td", "app-expand-cell");
    expandCell.append(toggle);

    const detailsRow = document.createElement("tr");
    detailsRow.className = "app-details-row";
    detailsRow.hidden = true;

    const detailsCell = newElement("td", "app-details-cell");
    detailsCell.colSpan = 3;

    const details = newElement("div", "app-details");
    details.id = detailsID;
    details.append(buildCommonGameDetails(game, users));
    detailsCell.append(details);
    detailsRow.append(detailsCell);

    toggle.addEventListener("click", () => {
        const expanded = toggle.getAttribute("aria-expanded") === "true";
        toggle.setAttribute("aria-expanded", String(!expanded));
        toggle.setAttribute(
            "aria-label",
            expanded
                ? `Show details for ${game.name ?? "app"}`
                : `Hide details for ${game.name ?? "app"}`,
        );
        detailsRow.hidden = expanded;
    });

    row.append(nameCell, storeCell, expandCell);

    return [row, detailsRow];
}

function buildCommonGameDetails(game, users) {
    const fragment = document.createDocumentFragment();
    const userCount = users.length;
    const ownedUsers = commonGamesUsersForIDs(users, game.ownedSteamIds);
    const wishlistedUsers = commonGamesUsersForIDs(
        users,
        game.wishlistedSteamIds,
    );
    const missingUsers = commonGamesMissingUsers(
        users,
        game.ownedSteamIds,
        game.wishlistedSteamIds,
    );

    if (game.headerImage) {
        const image = newElement(
            "img",
            "app-cell__image app-cell__image--large",
        );
        image.src = game.headerImage;
        image.alt = "";
        image.loading = "lazy";
        fragment.append(image);
    }

    if (
        typeof game.shortDescription === "string" &&
        game.shortDescription.trim() !== ""
    ) {
        fragment.append(
            newElement(
                "p",
                "app-details__description",
                game.shortDescription.trim(),
            ),
        );
    }

    const rows = newElement("dl", "details app-details__list");
    rows.append(
        detailRow("Release date", appReleaseLabel(game)),
        ...detailRowWithUsers(
            "Owned",
            `${formatCount(game.ownedCount ?? 0)} (${commonGamesPercent(
                game.ownedCount ?? 0,
                userCount,
            )}%)`,
            ownedUsers,
            "Users who own this game",
        ),
        ...detailRowWithUsers(
            "Wishlisted",
            `${formatCount(game.wishlistCount ?? 0)} (${commonGamesPercent(
                game.wishlistCount ?? 0,
                userCount,
            )}%)`,
            wishlistedUsers,
            "Users who have this game on their wishlist",
        ),
        ...detailRowWithUsers(
            "Missing",
            "",
            missingUsers,
            "Users who do not own or wishlist this game",
        ),
        detailRow("Price", appPriceDetail(game)),
        detailRow("Platforms", appPlatformsLabel(game)),
        buildAppWebsiteRow(game.website),
    );
    fragment.append(rows);

    return fragment;
}

function commonGamesPercent(count, total) {
    const numerator = Number(count);
    const denominator = Number(total);
    if (
        !Number.isFinite(numerator) ||
        !Number.isFinite(denominator) ||
        denominator <= 0
    ) {
        return 0;
    }
    return Math.round((numerator / denominator) * 100);
}

const COMMON_GAMES_PERCENT_STOPS = [
    { percent: 40, color: [220, 38, 38] }, // red
    { percent: 60, color: [249, 115, 22] }, // orange
    { percent: 80, color: [234, 179, 8] }, // yellow
    { percent: 100, color: [22, 163, 74] }, // green
];

function buildCommonGamesPercentBadge(percent) {
    const badge = newElement("span", "app-cell__percent", `${percent}%`);
    const value = Math.min(100, Math.max(0, Number(percent) || 0));
    const color = commonGamesPercentColor(value);

    badge.style.setProperty("--percent-color", formatRGBColor(color));
    badge.style.setProperty(
        "--percent-text",
        commonGamesPercentTextColor(color),
    );
    return badge;
}

function commonGamesPercentColor(percent) {
    if (percent <= 40) {
        return COMMON_GAMES_PERCENT_STOPS[0].color;
    }

    for (let index = 1; index < COMMON_GAMES_PERCENT_STOPS.length; index += 1) {
        const next = COMMON_GAMES_PERCENT_STOPS[index];
        if (percent <= next.percent) {
            const previous = COMMON_GAMES_PERCENT_STOPS[index - 1];
            const position =
                (percent - previous.percent) /
                (next.percent - previous.percent);
            return previous.color.map((channel, channelIndex) =>
                Math.round(
                    channel + (next.color[channelIndex] - channel) * position,
                ),
            );
        }
    }

    return COMMON_GAMES_PERCENT_STOPS[COMMON_GAMES_PERCENT_STOPS.length - 1]
        .color;
}

function formatRGBColor([red, green, blue]) {
    return `rgb(${red}, ${green}, ${blue})`;
}

function commonGamesPercentTextColor([red, green, blue]) {
    const luminance =
        0.2126 * linearColorChannel(red) +
        0.7152 * linearColorChannel(green) +
        0.0722 * linearColorChannel(blue);
    return luminance > 0.22 ? "#1b2430" : "#ffffff";
}

function linearColorChannel(value) {
    const channel = value / 255;
    return channel <= 0.03928
        ? channel / 12.92
        : ((channel + 0.055) / 1.055) ** 2.4;
}

function clearCommonGamesResultsView() {
    commonGamesResults.hidden = true;
    commonGamesResultsCount.textContent = "";
    commonGamesQueryUsers.replaceChildren();
    commonGamesResultsEmpty.hidden = true;
    commonGamesTableScroll.hidden = true;
    commonGamesGameRows.replaceChildren();
    commonGamesNote.hidden = true;
}

function invalidateCommonGamesResults() {
    commonGamesRequestToken += 1;
    commonGamesRequestController?.abort();
    commonGamesRequestController = null;
    commonGamesBusy = false;
    commonGamesState.results = null;
    commonGamesState.submitted = false;
    clearCommonGamesResultsView();
    commonGamesResults.setAttribute("aria-busy", "false");
    setCommonGamesStatus("");
    syncCommonGamesControls();
    syncCommonGamesURL();
}

function setCommonGamesSearchStatus(message, state) {
    commonGamesSearchStatus.textContent = message;
    if (state) {
        commonGamesSearchStatus.dataset.state = state;
    } else {
        delete commonGamesSearchStatus.dataset.state;
    }
}

function setCommonGamesStatus(message, state) {
    commonGamesStatus.textContent = message;
    if (state) {
        commonGamesStatus.dataset.state = state;
    } else {
        delete commonGamesStatus.dataset.state;
    }
}

function setCommonGamesSearchBusy(busy) {
    commonGamesSearchBusy = busy;
    commonGamesSearchInput.disabled = busy;
    commonGamesSearchButton.disabled = busy;
}

function isCommonGamesSearchCurrent(token) {
    return token === commonGamesSearchToken;
}

function isCommonGamesRequestCurrent(token) {
    return token === commonGamesRequestToken;
}

function enterDashboard() {
    void loadDashboard();
}

function leaveDashboard() {
    stopDashboardRefresh();
    dashboardController?.abort();
    dashboardController = null;
}

function onDashboardRoute() {
    return currentRoute === "dashboard";
}

function stopDashboardRefresh() {
    if (dashboardTimer !== null) {
        clearTimeout(dashboardTimer);
        dashboardTimer = null;
    }
}

function scheduleDashboardRefresh() {
    stopDashboardRefresh();
    if (!onDashboardRoute() || document.visibilityState !== "visible") {
        return;
    }
    dashboardTimer = setTimeout(() => {
        dashboardTimer = null;
        void loadDashboard();
    }, DASHBOARD_REFRESH_MS);
}

async function loadDashboard() {
    stopDashboardRefresh();
    dashboardController?.abort();
    const controller = new AbortController();
    dashboardController = controller;

    try {
        const response = await apiFetch("/api/dashboard", {
            headers: { Accept: "application/json" },
            signal: controller.signal,
        });
        if (controller.signal.aborted) {
            return;
        }
        if (!response.ok) {
            throw await readError(response);
        }

        const body = await response.json();
        if (controller.signal.aborted) {
            return;
        }

        renderDashboard(body);
        dashboardMetrics.setAttribute("aria-busy", "false");
        setDashboardStatus("");
        dashboardUpdated.textContent = `Updated at ${formatClock(new Date())}`;
    } catch (error) {
        if (isAbort(error) || controller.signal.aborted) {
            return;
        }
        dashboardMetrics.setAttribute("aria-busy", "false");
        setDashboardStatus(
            "Could not refresh the dashboard; retrying…",
            "error",
        );
    } finally {
        if (dashboardController === controller) {
            dashboardController = null;
        }
    }

    scheduleDashboardRefresh();
}

function renderDashboard(stats) {
    metricUsers.textContent = formatCount(stats?.totalSteamUsers);
    metricApps.textContent = formatCount(stats?.totalApps);
    metricRunning.textContent = formatCount(stats?.importJobs?.running);
    metricQueued.textContent = formatCount(stats?.importJobs?.queued);
}

function formatCount(value) {
    const number = Number(value);
    return Number.isFinite(number) ? numberFormatter.format(number) : "—";
}

function formatClock(date) {
    return date.toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
    });
}

function setDashboardStatus(message, state) {
    dashboardStatus.textContent = message;
    if (state) {
        dashboardStatus.dataset.state = state;
    } else {
        delete dashboardStatus.dataset.state;
    }
}

document.addEventListener("visibilitychange", () => {
    if (!onDashboardRoute()) {
        return;
    }
    if (document.visibilityState === "visible") {
        void loadDashboard();
    } else {
        stopDashboardRefresh();
        dashboardController?.abort();
        dashboardController = null;
    }
});

function enterLogin() {
    setLoginStatus("");
}

function enterCreateAccount() {
    setCreateAccountStatus("");
}

const pageLifecycles = {
    dashboard: { enter: enterDashboard, leave: leaveDashboard },
    import: { enter: enterImport, leave: leaveImport },
    search: { leave: leaveSearch },
    user: { enter: enterUser, leave: leaveUser },
    commonGames: { enter: enterCommonGames, leave: leaveCommonGames },
    login: { enter: enterLogin },
    createAccount: { enter: enterCreateAccount },
};

function onRouteChange(previous, next) {
    const previousPage = previous ? pageLifecycles[previous] : null;
    if (previousPage?.leave) {
        previousPage.leave();
    }
    const nextPage = pageLifecycles[next];
    if (nextPage?.enter) {
        nextPage.enter();
    }
}

function renderRoute() {
    if (!authState.loaded) {
        return;
    }

    const path = normalizePath(location.pathname);
    const routePath = ROUTES.has(path) ? path : "/";
    if (!ROUTES.has(path)) {
        history.replaceState({}, "", "/");
    }

    const redirect = authRedirectFor(routePath);
    if (redirect !== null) {
        history.replaceState({}, "", redirect);
        renderRoute();
        return;
    }

    updateAuthChrome();

    const route = ROUTES.get(routePath);
    const previous = currentRoute;
    const href = location.pathname + location.search + location.hash;
    const changed = previous !== route || currentHref !== href;
    currentRoute = route;
    currentHref = href;

    let activeSection = null;
    for (const section of viewSections) {
        const isActive = section.dataset.view === routePath;
        section.hidden = !isActive;
        if (isActive) {
            activeSection = section;
        }
    }

    for (const link of navLinks) {
        if (link.dataset.route === routePath) {
            link.setAttribute("aria-current", "page");
        } else {
            link.removeAttribute("aria-current");
        }
    }

    document.title = `${ROUTE_TITLES[route]} · Steam Decision Engine`;

    if (changed) {
        onRouteChange(previous, route);
    }

    if (previous !== route) {
        activeSection?.querySelector("h1")?.focus({ preventScroll: true });
    }
}

function normalizePath(pathname) {
    if (pathname === "/") {
        return "/";
    }
    return pathname.replace(/\/+$/, "") || "/";
}

function handleRouteClick(event) {
    if (
        event.defaultPrevented ||
        event.button !== 0 ||
        event.metaKey ||
        event.ctrlKey ||
        event.shiftKey ||
        event.altKey
    ) {
        return;
    }

    const anchor = event.target.closest("a[data-route]");
    if (!anchor) {
        return;
    }

    const url = new URL(anchor.href, location.href);
    if (url.origin !== location.origin) {
        return;
    }

    event.preventDefault();

    const target = url.pathname + url.search + url.hash;
    const current = location.pathname + location.search + location.hash;
    if (target !== current) {
        history.pushState({}, "", target);
    }
    renderRoute();
}

document.addEventListener("click", handleRouteClick);
window.addEventListener("popstate", renderRoute);
void bootstrap();
