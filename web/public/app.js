"use strict";

const POLL_INTERVAL_MS = 2000;
const POLL_TIMEOUT_MS = 5 * 60 * 1000;
const DASHBOARD_REFRESH_MS = 5000;
const PROFILE_PREFIX = "https://steamcommunity.com/id/";
const PROFILE_ID_PREFIX = "https://steamcommunity.com/profiles/";

const MAX_COMMON_GAME_USERS = 25;
const MAX_CUSTOM_USERNAME_LENGTH = 255; // mirrors the server's name limit
const COMMON_GAMES_LIMIT_MIN = 1;
const COMMON_GAMES_LIMIT_MAX = 100;
const COMMON_GAMES_LIMIT_DEFAULT = 10;

const ROUTES = new Map([
    ["/", "commonGames"],
    ["/admin-panel", "dashboard"],
    ["/import", "import"],
    ["/user", "user"],
    ["/group", "group"],
    ["/login", "login"],
    ["/create-account", "createAccount"],
]);

// The standalone Search pages were merged into the combined Users and
// Groups pages. Old links keep working by redirecting, carrying the query.
const LEGACY_ROUTE_REDIRECTS = new Map([
    ["/search", "/user"],
    ["/search-groups", "/group"],
]);

const ROUTE_TITLES = {
    dashboard: "Admin Panel",
    import: "Import Steam User",
    user: "Users",
    group: "Groups",
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
const customUsernameRow = document.querySelector("#custom-username-row");
const customUsernameInput = document.querySelector("#custom-username");
const resultStatusBadge = document.querySelector("#result-status-badge");
const resultAvatarRow = document.querySelector("#result-avatar-row");
const resultAvatar = document.querySelector("#result-avatar");
const resultPersona = document.querySelector("#result-persona");
const resultOwned = document.querySelector("#result-owned");
const resultWishlist = document.querySelector("#result-wishlist");
const resultLastImport = document.querySelector("#result-last-import");
const resultImportError = document.querySelector("#result-import-error");
const resultActions = document.querySelector("#result-actions");
const importButton = document.querySelector("#import-button");
const updateButton = document.querySelector("#update-button");

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

// Admin Panel import jobs list.
const importJobsStatus = document.querySelector("#import-jobs-status");
const importJobsToolbar = document.querySelector("#import-jobs-toolbar");
const importJobsCount = document.querySelector("#import-jobs-count");
const importJobsPageSize = document.querySelector("#import-jobs-page-size");
const importJobsResultsPanel = document.querySelector(
    "#import-jobs-results-panel",
);
const importJobsResults = document.querySelector("#import-jobs-results");
const importJobsPagination = document.querySelector(
    "#import-jobs-pagination",
);

const searchControls = document.querySelector("#search-controls");
const searchSort = document.querySelector("#search-sort");
const searchDirection = document.querySelector("#search-direction");
const searchWishlist = document.querySelector("#search-wishlist");
const searchReset = document.querySelector("#search-reset");
const searchStatus = document.querySelector("#search-status");
const searchToolbar = document.querySelector("#search-toolbar");
const searchCount = document.querySelector("#search-count");
const searchPageSize = document.querySelector("#search-page-size");
const searchResultsPanel = document.querySelector("#search-results-panel");
const searchResults = document.querySelector("#search-results");
const searchPagination = document.querySelector("#search-pagination");

const searchGroupsControls = document.querySelector("#search-groups-controls");
const searchGroupsSort = document.querySelector("#search-groups-sort");
const searchGroupsDirection = document.querySelector(
    "#search-groups-direction",
);
const searchGroupsReset = document.querySelector("#search-groups-reset");
const searchGroupsStatus = document.querySelector("#search-groups-status");
const searchGroupsToolbar = document.querySelector("#search-groups-toolbar");
const searchGroupsCount = document.querySelector("#search-groups-count");
const searchGroupsPageSize = document.querySelector("#search-groups-page-size");
const searchGroupsResultsPanel = document.querySelector(
    "#search-groups-results-panel",
);
const searchGroupsResults = document.querySelector("#search-groups-results");
const searchGroupsPagination = document.querySelector(
    "#search-groups-pagination",
);

const userForm = document.querySelector("#user-form");
const userInput = document.querySelector("#user-name");
const userSubmit = document.querySelector("#user-submit");
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
const userGroups = document.querySelector("#user-groups");
const userGroupsEmpty = document.querySelector("#user-groups-empty");
const userGroupsList = document.querySelector("#user-groups-list");
const userGroupsStatus = document.querySelector("#user-groups-status");
const userGroupAddForm = document.querySelector("#user-group-add-form");
const userGroupAddInput = document.querySelector("#user-group-add-name");
const userGroupAddButton = document.querySelector("#user-group-add-button");
const userGroupAddResults = document.querySelector("#user-group-add-results");

const groupForm = document.querySelector("#group-form");
const groupInput = document.querySelector("#group-name");
const groupSubmit = document.querySelector("#group-submit");
const groupStatusLine = document.querySelector("#group-status");
const groupSummary = document.querySelector("#group-summary");
const groupStatusBadge = document.querySelector("#group-status-badge");
const groupSummaryDetails = document.querySelector("#group-summary-details");
const groupMembers = document.querySelector("#group-members");
const groupMembersControls = document.querySelector("#group-members-controls");
const groupMembersSort = document.querySelector("#group-members-sort");
const groupMembersDirection = document.querySelector(
    "#group-members-direction",
);
const groupMembersCount = document.querySelector("#group-members-count");
const groupMembersPanel = document.querySelector("#group-members-panel");
const groupMemberRows = document.querySelector("#group-member-rows");
const createGroupButton = document.querySelector("#create-group-button");
const createGroupDialog = document.querySelector("#create-group-dialog");
const groupCreateForm = document.querySelector("#group-create-form");
const groupCreateInput = document.querySelector("#group-create-name");
const groupCreateButton = document.querySelector("#group-create-button");
const groupCreateCancel = document.querySelector("#group-create-cancel");
const groupCreateStatus = document.querySelector("#group-create-status");
const groupRenameButton = document.querySelector("#group-rename-button");
const groupDeleteButton = document.querySelector("#group-delete-button");
const groupRenameForm = document.querySelector("#group-rename-form");
const groupRenameInput = document.querySelector("#group-rename-name");
const groupRenameSave = document.querySelector("#group-rename-save");
const groupRenameCancel = document.querySelector("#group-rename-cancel");
const groupRenameStatus = document.querySelector("#group-rename-status");
const groupAddMembers = document.querySelector("#group-add-members");
const groupAddForm = document.querySelector("#group-add-form");
const groupAddInput = document.querySelector("#group-add-name");
const groupAddButton = document.querySelector("#group-add-button");
const groupAddStatus = document.querySelector("#group-add-status");
const groupAddResults = document.querySelector("#group-add-results");

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
const commonGamesUserResultsSection = document.querySelector(
    "#common-games-user-results-section",
);
const commonGamesSearchResults = document.querySelector(
    "#common-games-search-results",
);
const commonGamesGroupResultsSection = document.querySelector(
    "#common-games-group-results-section",
);
const commonGamesGroupResults = document.querySelector(
    "#common-games-group-results",
);
const commonGamesSelectedPanel = document.querySelector(
    "#common-games-selected-panel",
);
const commonGamesSelectedCount = document.querySelector(
    "#common-games-selected-count",
);
const commonGamesSelectedEmpty = document.querySelector(
    "#common-games-selected-empty",
);
const commonGamesSelectedGroupsSection = document.querySelector(
    "#common-games-selected-groups-section",
);
const commonGamesSelectedGroups = document.querySelector(
    "#common-games-selected-groups",
);
const commonGamesSelectedUsersSection = document.querySelector(
    "#common-games-selected-users-section",
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
// The last lookup payload plus the normalized name that produced it. Import
// and Update send both so the server can reject a stale preview with a 409.
let currentLookup = null;
let currentLookupName = "";
// "id" when the lookup input was a SteamID64; "vanity" otherwise. Import
// requests built in ID mode skip vanityUrl entirely.
let currentLookupMode = "vanity";

let dashboardTimer = null;
let dashboardController = null;

// The job rows shown under the At a Glance metrics. expandedImportJobs keeps
// opened rows open across the 5-second refresh, keyed by String(job.id).
let importJobs = [];
let importJobsPage = 1;
let importJobsRowsPerPage = 10;
const expandedImportJobs = new Set();

let searchToken = 0;
let searchController = null;
let searchBusy = false;
let searchUsers = [];
let searchPage = 1;
let searchRowsPerPage = 10;

const SEARCH_SORT_DEFAULT_DIRECTIONS = {
    name: "asc",
    updated: "desc",
    owned: "desc",
    wishlist: "desc",
};

let searchGroupsToken = 0;
let searchGroupsController = null;
let searchGroupsBusy = false;
let searchGroups = [];
let searchGroupsPage = 1;
let searchGroupsRowsPerPage = 10;

const GROUP_SORT_DEFAULT_DIRECTIONS = {
    name: "asc",
    members: "desc",
};

const APP_PAGE_SIZES = [25, 50, 100];
const SEARCH_PAGE_SIZES = [10, 25, 50];

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

let userGroupSearchToken = 0;
let userGroupSearchController = null;
let userGroupSearchResults = [];
// Only one View User group add/remove runs at a time.
let userGroupsBusy = false;

let groupToken = 0;
let groupController = null;
const groupState = {
    name: "",
    data: null,
};
// One flag for every group mutation (create, rename, delete, add, remove) so
// only one runs at a time.
let groupMutationBusy = false;

let groupAddToken = 0;
let groupAddController = null;
let groupAddUsers = [];

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
    // Keyed by String(groupId). Groups stay listed as groups; their members
    // are expanded into distinct Steam IDs only when counting or submitting.
    selectedGroups: new Map(),
    searchUsers: [],
    searchGroups: [],
    results: null,
    submitted: false,
};

form.addEventListener("submit", (event) => {
    event.preventDefault();
    void search();
});

importButton.addEventListener("click", () => void startImport(false));
updateButton.addEventListener("click", () => void startImport(true));

// The custom-username field belongs to the ID path only: show it while the
// current input reads as a SteamID64 and clear it on every switch back.
input.addEventListener("input", () => {
    const isSteamID = detectSteamID(normalizeName(input.value));
    customUsernameRow.hidden = !isSteamID;
    if (!isSteamID) {
        customUsernameInput.value = "";
    }
});

searchSort.addEventListener("change", () => {
    searchDirection.value =
        SEARCH_SORT_DEFAULT_DIRECTIONS[searchSort.value] ?? "asc";
    searchPage = 1;
    renderSearchPipeline();
});
searchDirection.addEventListener("change", () => {
    searchPage = 1;
    renderSearchPipeline();
});
searchWishlist.addEventListener("change", () => {
    searchPage = 1;
    renderSearchPipeline();
});
searchPageSize.addEventListener("change", () => {
    searchRowsPerPage = searchPageSizeFromValue(searchPageSize.value);
    searchPage = 1;
    renderSearchPipeline();
});
searchReset.addEventListener("click", () => {
    resetSearchControls();
    renderSearchPipeline();
});

searchGroupsSort.addEventListener("change", () => {
    searchGroupsDirection.value =
        GROUP_SORT_DEFAULT_DIRECTIONS[searchGroupsSort.value] ?? "asc";
    searchGroupsPage = 1;
    renderGroupSearchResults();
});
searchGroupsDirection.addEventListener("change", () => {
    searchGroupsPage = 1;
    renderGroupSearchResults();
});
searchGroupsPageSize.addEventListener("change", () => {
    searchGroupsRowsPerPage = searchPageSizeFromValue(
        searchGroupsPageSize.value,
    );
    searchGroupsPage = 1;
    renderGroupSearchResults();
});
searchGroupsReset.addEventListener("click", () => {
    resetGroupSearchControls();
    renderGroupSearchResults();
});

importJobsPageSize.addEventListener("change", () => {
    importJobsRowsPerPage = searchPageSizeFromValue(importJobsPageSize.value);
    importJobsPage = 1;
    renderImportJobs();
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

userGroupAddForm.addEventListener("submit", (event) => {
    event.preventDefault();
    void performUserGroupSearch();
});

userGroupAddResults.addEventListener("click", (event) => {
    const target = event.target.closest("button[data-user-group-add-id]");
    if (target) {
        void addUserToGroup(target.dataset.userGroupAddId ?? "");
    }
});

userGroupsList.addEventListener("click", (event) => {
    const target = event.target.closest("button[data-user-group-remove-id]");
    if (target) {
        void removeUserFromGroup(target.dataset.userGroupRemoveId ?? "");
    }
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

groupForm.addEventListener("submit", (event) => {
    event.preventDefault();
    const name = groupInput.value.trim();
    if (!name) {
        setGroupStatus("Enter a group name.", "error");
        groupInput.focus();
        return;
    }
    pushGroupURL(name);
});

createGroupButton.addEventListener("click", () => {
    openCreateGroupDialog();
});

groupCreateForm.addEventListener("submit", (event) => {
    event.preventDefault();
    void createGroup();
});

groupCreateCancel.addEventListener("click", () => {
    createGroupDialog.close();
});

// A click whose target is the dialog itself came from the backdrop.
createGroupDialog.addEventListener("click", (event) => {
    if (event.target === createGroupDialog) {
        createGroupDialog.close();
    }
});

createGroupDialog.addEventListener("close", () => {
    createGroupButton.focus();
});

groupMembersSort.addEventListener("change", () => {
    groupMembersDirection.value =
        SEARCH_SORT_DEFAULT_DIRECTIONS[groupMembersSort.value] ?? "asc";
    renderGroupMembers();
});
groupMembersDirection.addEventListener("change", renderGroupMembers);

groupRenameButton.addEventListener("click", () => {
    setRenameFormOpen(groupRenameForm.hidden);
});
groupRenameCancel.addEventListener("click", () => {
    setRenameFormOpen(false);
    groupRenameButton.focus();
});
groupRenameForm.addEventListener("submit", (event) => {
    event.preventDefault();
    void renameGroup();
});

groupDeleteButton.addEventListener("click", () => {
    void deleteGroup();
});

groupAddForm.addEventListener("submit", (event) => {
    event.preventDefault();
    void performGroupAddSearch();
});

groupAddResults.addEventListener("click", (event) => {
    const target = event.target.closest("button[data-group-add-steam-id]");
    if (target) {
        void addGroupMember(target.dataset.groupAddSteamId ?? "");
    }
});

groupMemberRows.addEventListener("click", (event) => {
    const target = event.target.closest("button[data-group-remove-steam-id]");
    if (target) {
        void removeGroupMember(target.dataset.groupRemoveSteamId ?? "");
    }
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

commonGamesSearchResultsPanel.addEventListener("click", (event) => {
    const userTarget = event.target.closest("button[data-add-steam-id]");
    if (userTarget) {
        const steamID = userTarget.dataset.addSteamId ?? "";
        const user = commonGamesState.searchUsers.find(
            (candidate) => String(candidate.steamId) === steamID,
        );
        if (user) {
            addCommonGamesUser(user);
        }
        return;
    }

    const groupTarget = event.target.closest("button[data-add-group-id]");
    if (groupTarget) {
        const groupID = groupTarget.dataset.addGroupId ?? "";
        const group = commonGamesState.searchGroups.find(
            (candidate) => String(candidate.groupId) === groupID,
        );
        if (group) {
            addCommonGamesGroup(group);
        }
    }
});

commonGamesSelectedPanel.addEventListener("click", (event) => {
    const userTarget = event.target.closest("button[data-remove-steam-id]");
    if (userTarget) {
        removeCommonGamesUser(userTarget.dataset.removeSteamId ?? "");
        return;
    }

    const groupTarget = event.target.closest("button[data-remove-group-id]");
    if (groupTarget) {
        removeCommonGamesGroup(groupTarget.dataset.removeGroupId ?? "");
    }
});

// search performs the read-only lookup. A 200 renders the state the server
// computed; a 404 means neither the database nor Steam knows the name. A
// lookup never posts, so it can never write a row or queue a job.
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

    // Remember how this lookup was entered so Import/Update build the right
    // request body; keep the label field in step with the detected mode.
    currentLookupMode = detectSteamID(name) ? "id" : "vanity";
    customUsernameRow.hidden = currentLookupMode !== "id";

    setBusy(true);
    setStatus(`Looking up “${name}”…`);

    try {
        const response = await apiFetch(lookupURL(name), {
            headers: { Accept: "application/json" },
            signal: controller.signal,
        });
        if (!isCurrent(token)) {
            return;
        }

        if (response.status === 200) {
            const body = await response.json();
            if (!isCurrent(token)) {
                return;
            }
            currentLookupName = name;
            applyLookup(body, token);
            prefillCustomUsername(body);
            return;
        }

        if (response.status === 404) {
            setStatus(
                "Steam user not found. Check the name and try again.",
                "error",
            );
            return;
        }

        throw await readError(response);
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
            "The import is taking longer than expected. Look up the user again to see its status.",
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
                handleJobTerminal(currentJob, token);
            } else {
                updateLatestImport(currentJob);
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

// UI copy per import state from GET /api/steamusers. The tones reuse the
// shared .badge styles.
const IMPORT_STATE_META = {
    not_imported: { label: "Not imported", tone: "none" },
    queued: { label: "Queued", tone: "queued" },
    running: { label: "Importing", tone: "running" },
    up_to_date: { label: "Up to date", tone: "succeeded" },
    out_of_date: { label: "Out of date", tone: "out-of-date" },
};

// applyLookup renders a lookup payload: the summary, the badge, the action
// buttons, and polling for an in-flight job. options.keepJob leaves a visible
// completed job panel alone; options.status overrides the state message.
function applyLookup(body, token, options = {}) {
    currentLookup = body;
    renderResultSummary(body);
    applyImportState(body, token, options);
}

function renderResultSummary(body) {
    const user = body.steamUser ?? {};
    const latest = body.latestImport ?? null;

    resultName.textContent = user.name ?? "";
    // Steam IDs exceed JavaScript's safe integer range, so keep the value as
    // text and never convert it to a number.
    steamID.textContent =
        user.steamId === null || user.steamId === undefined
            ? ""
            : String(user.steamId);
    resultPersona.textContent = user.personaName || "—";
    if (body.source === "database") {
        resultSource.textContent = user.customUsername
            ? "Stored in the database (custom username)"
            : "Stored in the database";
    } else {
        resultSource.textContent = "Resolved with Steam";
    }
    resultOwned.textContent =
        body.ownedAppCount === null || body.ownedAppCount === undefined
            ? "—"
            : formatCount(body.ownedAppCount);
    resultWishlist.textContent =
        body.wishlistAppCount === null || body.wishlistAppCount === undefined
            ? "—"
            : formatCount(body.wishlistAppCount);
    resultLastImport.textContent = lastImportText(latest);

    // Users without profile data have no avatar, matching View User.
    const avatar = user.avatarMedium || user.avatarFull || user.avatar;
    if (avatar) {
        resultAvatar.src = avatar;
        resultAvatarRow.hidden = false;
    } else {
        resultAvatar.removeAttribute("src");
        resultAvatarRow.hidden = true;
    }

    if (latest?.error) {
        resultImportError.textContent = latest.error;
        resultImportError.hidden = false;
    } else {
        resultImportError.textContent = "";
        resultImportError.hidden = true;
    }

    const meta = IMPORT_STATE_META[body.importState] ?? {
        label: String(body.importState ?? "Unknown"),
        tone: "unknown",
    };
    resultStatusBadge.textContent = meta.label;
    resultStatusBadge.dataset.status = meta.tone;

    result.hidden = false;
}

// applyImportState shows exactly the action the server state allows: an
// active job polls with both buttons hidden, a never-imported user offers
// Import, and a stale or failed one offers Update.
function applyImportState(body, token, options = {}) {
    const state = body.importState;
    const latest = body.latestImport ?? null;

    const showImport = state === "not_imported";
    const showUpdate = state === "out_of_date";
    importButton.hidden = !showImport;
    updateButton.hidden = !showUpdate;
    resultActions.hidden = !showImport && !showUpdate;

    if ((state === "queued" || state === "running") && latest) {
        startPolling(latest, token);
    } else if (!options.keepJob) {
        jobSection.hidden = true;
    }

    if (options.status !== undefined) {
        setStatus(options.status, options.statusState);
    } else {
        setStatus(stateMessage(state, latest));
    }
}

function stateMessage(state, latest) {
    switch (state) {
        case "not_imported":
            return "Not in the database yet. Check the Steam profile, then choose Import User.";
        case "queued":
            return "Import queued.";
        case "running":
            return "Import in progress…";
        case "up_to_date":
            return "Up to date.";
        case "out_of_date":
            return latest?.status === "failed"
                ? "The last import failed. Choose Update user to try again."
                : "The last successful import is out of date. Choose Update user to refresh it.";
        default:
            return "";
    }
}

// startImport posts the name and Steam ID from the last lookup. force is the
// Update user path: the server always starts a fresh job unless one is
// already active. A 409 means the name no longer matches the previewed ID.
async function startImport(force) {
    if (!currentLookup?.steamUser) {
        return;
    }

    const token = ++importToken;
    lookupController?.abort();
    stopPolling();
    lastJob = null;

    const controller = new AbortController();
    lookupController = controller;

    // ID mode imports skip vanity resolution entirely: the previewed (or
    // entered) Steam ID is the identifier and the optional label rides
    // along. Vanity mode keeps today's body.
    const steamIDValue = String(currentLookup.steamUser.steamId ?? "");
    const request =
        currentLookupMode === "id"
            ? {
                  steamId: steamIDValue,
                  customUsername: customUsernameInput.value.trim(),
                  force,
              }
            : {
                  vanityUrl: currentLookupName,
                  steamId: steamIDValue,
                  force,
              };

    setBusy(true);
    setActionsBusy(true);
    setStatus(force ? "Starting update…" : "Starting import…");

    try {
        const response = await apiFetch("/api/steamuser", {
            method: "POST",
            headers: {
                Accept: "application/json",
                "Content-Type": "application/json",
            },
            body: JSON.stringify(request),
            signal: controller.signal,
        });
        if (!isCurrent(token)) {
            return;
        }

        if (response.status === 404 || response.status === 409) {
            const error = await readError(response);
            setStatus(error.message, "error");
            // In ID mode a 409 is the taken-username guidance: keep the
            // label focused for the retry.
            if (response.status === 409 && currentLookupMode === "id") {
                customUsernameInput.focus();
            }
            return;
        }
        if (!response.ok) {
            throw await readError(response);
        }

        const body = await response.json();
        if (!isCurrent(token)) {
            return;
        }

        const job = body.importJob ?? null;
        const payload = {
            ...currentLookup,
            steamUser: body.steamUser ?? currentLookup.steamUser,
            source: "database",
            latestImport: job,
            hasActiveJob: isJobActive(job),
        };
        const state = stateFromJob(job);
        if (state) {
            payload.importState = state;
        }
        // An active state starts polling from applyImportState; only a
        // terminal job needs the completion handler directly.
        applyLookup(payload, token, { keepJob: true });
        if (job && isJobTerminal(job)) {
            handleJobTerminal(job, token);
        }
    } catch (error) {
        if (isAbort(error) || !isCurrent(token)) {
            return;
        }
        setStatus(errorMessage(error), "error");
    } finally {
        if (isCurrent(token)) {
            setBusy(false);
            setActionsBusy(false);
            lookupController = null;
        }
    }
}

// handleJobTerminal folds a finished job into the rendered state. A success
// keeps the completed job panel visible and re-reads the lookup for fresh
// counts; a failure offers Update user for a retry.
function handleJobTerminal(job, token) {
    if (!currentLookup) {
        return;
    }
    const succeeded = job.status === "succeeded";
    const payload = {
        ...currentLookup,
        latestImport: job,
        hasActiveJob: false,
        importState: succeeded ? "up_to_date" : "out_of_date",
    };
    if (succeeded) {
        applyLookup(payload, token, {
            keepJob: true,
            status: "Import complete.",
        });
        void refreshCounts(token);
    } else {
        applyLookup(payload, token, {
            keepJob: true,
            status: "Import failed. Choose Update user to retry.",
            statusState: "error",
        });
    }
}

// refreshCounts silently re-reads the lookup after a successful import so
// the owned and wishlist counts come from the database, keeping the
// completed job panel on screen.
async function refreshCounts(token) {
    if (!currentLookupName) {
        return;
    }

    const controller = new AbortController();
    try {
        const response = await apiFetch(lookupURL(currentLookupName), {
            headers: { Accept: "application/json" },
            signal: controller.signal,
        });
        if (!isCurrent(token) || !onImportRoute()) {
            return;
        }
        if (response.status !== 200) {
            return;
        }
        const body = await response.json();
        if (!isCurrent(token)) {
            return;
        }
        applyLookup(body, token, {
            keepJob: true,
            status: "Import complete.",
        });
    } catch {
        // The pre-import counts stay; the next lookup refreshes them.
    }
}

// updateLatestImport keeps the lookup payload and the summary's last-import
// row in step with an active job while polling.
function updateLatestImport(job) {
    if (!currentLookup) {
        return;
    }
    currentLookup = {
        ...currentLookup,
        latestImport: job,
        hasActiveJob: isJobActive(job),
    };
    resultLastImport.textContent = lastImportText(job);
}

// stateFromJob maps a job status to the import state it implies right now.
function stateFromJob(job) {
    switch (job?.status) {
        case "queued":
            return "queued";
        case "running":
            return "running";
        case "succeeded":
            return "up_to_date";
        case "failed":
            return "out_of_date";
        default:
            return null;
    }
}

function isJobActive(job) {
    return Boolean(
        job && (job.status === "queued" || job.status === "running"),
    );
}

function isJobTerminal(job) {
    return Boolean(
        job && (job.status === "succeeded" || job.status === "failed"),
    );
}

function lastImportText(job) {
    if (!job) {
        return "—";
    }
    const labels = {
        queued: "Queued",
        running: "Started",
        succeeded: "Succeeded",
        failed: "Failed",
    };
    const label = labels[job.status] ?? String(job.status ?? "Unknown");
    const when = job.finishedAt || job.startedAt || job.createdAt;
    return when ? `${label} ${formatDateTime(when)}` : label;
}

const importDateFormatter = new Intl.DateTimeFormat([], {
    dateStyle: "medium",
    timeStyle: "short",
});

function formatDateTime(value) {
    if (!value) {
        return "";
    }
    const date = new Date(value);
    return Number.isNaN(date.getTime())
        ? String(value)
        : importDateFormatter.format(date);
}

function resetView() {
    currentLookup = null;
    currentLookupName = "";
    currentLookupMode = "vanity";

    result.hidden = true;
    jobSection.hidden = true;

    resultStatusBadge.textContent = "";
    resultStatusBadge.dataset.status = "none";
    resultName.textContent = "";
    steamID.textContent = "";
    resultPersona.textContent = "";
    resultSource.textContent = "";
    resultOwned.textContent = "";
    resultWishlist.textContent = "";
    resultLastImport.textContent = "";

    resultAvatar.removeAttribute("src");
    resultAvatarRow.hidden = true;

    resultImportError.textContent = "";
    resultImportError.hidden = true;

    importButton.hidden = true;
    updateButton.hidden = true;
    resultActions.hidden = true;
    setActionsBusy(false);

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

function setActionsBusy(busy) {
    importButton.disabled = busy;
    updateButton.disabled = busy;
    customUsernameInput.disabled = busy;
}

function normalizeName(rawValue) {
    let name = rawValue.trim();
    if (name.toLowerCase().startsWith(PROFILE_PREFIX)) {
        name = name.slice(PROFILE_PREFIX.length);
    } else if (name.toLowerCase().startsWith(PROFILE_ID_PREFIX)) {
        name = name.slice(PROFILE_ID_PREFIX.length);
    }
    return name.replace(/\/+$/, "").trim();
}

// detectSteamID reads normalized input as a SteamID64 when it is pure
// digits. The comparison stays on the string — IDs exceed JavaScript's safe
// integer range — and the server does the authoritative validation.
function detectSteamID(value) {
    return /^\d{1,19}$/.test(value);
}

// prefillCustomUsername seeds the label with the looked-up persona name
// after a first-import ID search — a read-only Steam preview of an
// unstored user — when the field is still blank, so the stored name starts
// as the person's real display name instead of the bare ID. Stored rows are
// left alone: a blank label there means "keep the existing name", and
// prefilling would silently rename the user on the next Update click.
// Anything the user already typed always wins.
function prefillCustomUsername(body) {
    if (currentLookupMode !== "id" || body?.source !== "steam") {
        return;
    }
    if (customUsernameInput.value.trim() !== "") {
        return;
    }
    const persona = body?.steamUser?.personaName;
    if (!persona) {
        return;
    }
    customUsernameInput.value = [...persona]
        .slice(0, MAX_CUSTOM_USERNAME_LENGTH)
        .join("");
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

// Shows the combined Users page's search state for the given term. The term
// also arrives from loadUser when the exact by-name lookup misses.
async function performSearch(term) {
    const token = ++searchToken;
    searchController?.abort();
    const controller = new AbortController();
    searchController = controller;

    const name = String(term ?? "").trim();
    if (!name) {
        clearSearchResults();
        setSearchStatus("Enter a Steam name.", "error");
        userInput.focus();
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
    searchToolbar.hidden = true;
    searchCount.textContent = "";
    searchCount.hidden = true;
    searchPagination.replaceChildren();
}

function renderSearchPipeline() {
    if (searchUsers.length === 0) {
        searchResultsPanel.hidden = true;
        searchControls.hidden = true;
        searchToolbar.hidden = true;
        searchPagination.replaceChildren();
        return;
    }

    searchControls.hidden = false;
    searchPageSize.value = String(searchRowsPerPage);
    const visible = visibleSearchUsers();
    const totalPages = totalPageCount(visible.length, searchRowsPerPage);
    searchPage = clampPage(searchPage, totalPages);
    const start = (searchPage - 1) * searchRowsPerPage;
    const pageUsers = visible.slice(start, start + searchRowsPerPage);

    searchResults.replaceChildren(
        ...pageUsers.flatMap((user, index) =>
            buildSteamUserRows(user, index, { includeGroups: true }),
        ),
    );
    searchToolbar.hidden = false;
    searchResultsPanel.hidden = visible.length === 0;
    searchCount.hidden = false;

    if (visible.length === 0) {
        searchCount.textContent = "No users match the current filters.";
        searchPagination.replaceChildren();
        return;
    }

    const hiddenCount = searchUsers.length - visible.length;
    const noun = visible.length === 1 ? "user" : "users";
    searchCount.textContent = `Page ${searchPage} of ${totalPages} · ${formatCount(
        visible.length,
    )} ${noun}`;
    if (hiddenCount > 0) {
        searchCount.textContent += ` · ${formatCount(hiddenCount)} hidden by filters`;
    }
    renderPagination(searchPagination, searchPage, totalPages, goToSearchPage);
}

function goToSearchPage(page) {
    const totalPages = totalPageCount(
        visibleSearchUsers().length,
        searchRowsPerPage,
    );
    if (
        !Number.isInteger(page) ||
        page < 1 ||
        page > totalPages ||
        page === searchPage
    ) {
        return;
    }
    searchPage = page;
    renderSearchPipeline();
}

function visibleSearchUsers() {
    return sortSteamUsers(
        searchUsers.filter(matchesSearchFilters),
        searchSort.value,
        searchDirection.value,
    );
}

// Returns a sorted copy. Ties fall back to name ascending, then Steam ID, so the
// order is stable regardless of direction.
function sortSteamUsers(users, sort, direction) {
    const multiplier = direction === "desc" ? -1 : 1;

    return [...users].sort((left, right) => {
        const primary = compareSearchField(left, right, sort);
        if (primary !== 0) {
            return primary * multiplier;
        }
        const byName = compareSearchNames(left, right);
        if (byName !== 0) {
            return byName;
        }
        return String(left.steamId).localeCompare(String(right.steamId));
    });
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
    searchPage = 1;
}

// Builds the main row and hidden details row for a Steam user, as shown in the
// Search Users table. options.idPrefix sets the details element id prefix, and
// options.actionCell, when given, is called with the user and must return a
// <td> placed before the expand cell.
function buildSteamUserRows(user, index, options = {}) {
    const idPrefix = options.idPrefix ?? "search-user-details";
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
    // Custom-labeled users (imported by Steam ID) get a quiet badge beside
    // the name so the handle is not mistaken for a real vanity. Group member
    // rows reuse this builder and get it for free.
    const nameRow = newElement("div", "app-cell__name-row");
    nameRow.append(nameLink);
    if (user.customUsername) {
        nameRow.append(badge("Custom name", "none"));
    }
    info.append(nameRow);

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

    const detailsID = `${idPrefix}-${user.steamId ?? "unknown"}-${index}`;
    const actionCell = options.actionCell ? options.actionCell(user) : null;

    const detailsRow = document.createElement("tr");
    detailsRow.className = "app-details-row";
    detailsRow.hidden = true;

    const detailsCell = newElement("td", "app-details-cell");
    detailsCell.colSpan = actionCell ? 4 : 3;

    const details = newElement("div", "app-details");
    details.id = detailsID;
    details.append(
        buildSearchResultDetails(user, {
            includeGroups: options.includeGroups === true,
        }),
    );
    detailsCell.append(details);
    detailsRow.append(detailsCell);

    const expandCell = buildExpandToggle(
        user.name ?? "user",
        detailsID,
        detailsRow,
    );

    row.append(nameCell, statusCell);
    if (actionCell) {
        row.append(actionCell);
    }
    row.append(expandCell);

    return [row, detailsRow];
}

// Returns the expand cell for a table row: a chevron button that shows or hides
// detailsRow and keeps aria-expanded and the label in sync. options.expanded
// opens the row as it is built, and options.onToggle reports each click so a
// caller can remember the state across a re-render.
function buildExpandToggle(label, detailsID, detailsRow, options = {}) {
    const toggle = newElement("button", "button--secondary app-expand");
    toggle.type = "button";

    const expanded = options.expanded === true;
    toggle.setAttribute("aria-expanded", String(expanded));
    toggle.setAttribute("aria-controls", detailsID);
    toggle.setAttribute(
        "aria-label",
        expanded ? `Hide details for ${label}` : `Show details for ${label}`,
    );
    detailsRow.hidden = !expanded;

    const chevron = newElement("span", "app-expand__chevron");
    chevron.setAttribute("aria-hidden", "true");
    toggle.append(chevron);

    toggle.addEventListener("click", () => {
        const nextExpanded = toggle.getAttribute("aria-expanded") !== "true";
        toggle.setAttribute("aria-expanded", String(nextExpanded));
        toggle.setAttribute(
            "aria-label",
            nextExpanded
                ? `Hide details for ${label}`
                : `Show details for ${label}`,
        );
        detailsRow.hidden = !nextExpanded;
        options.onToggle?.(nextExpanded);
    });

    const expandCell = newElement("td", "app-expand-cell");
    expandCell.append(toggle);
    return expandCell;
}

function buildSearchResultDetails(user, options = {}) {
    const details = newElement("dl", "details app-details__list");
    // Steam IDs exceed JavaScript's safe integer range, so keep them as text.
    details.append(
        detailRow("Steam user name", user.name ?? ""),
        detailRow("Steam ID", String(user.steamId ?? "")),
        detailRow("Personaname", user.personaName || "—"),
        buildProfileURLRow(user.profileUrl),
    );
    if (options.includeGroups) {
        details.append(buildUserGroupsDetailRow(user.groups));
    }
    return details;
}

function buildUserGroupsDetailRow(groups) {
    const visibleGroups = Array.isArray(groups) ? groups : [];
    if (visibleGroups.length === 0) {
        return detailRow("Groups", "—");
    }

    const row = newElement("div", "details__row details__row--chips");
    const value = newElement("dd", "details__chips");
    value.append(...visibleGroups.map(buildGroupChip));
    row.append(newElement("dt", "", "Groups"), value);
    return row;
}

// Reuses the Common Games chip styling so group chips look the same everywhere.
function buildGroupChip(group) {
    const chip = newElement("span", "query-users__chip");
    const link = newElement("a", "query-users__name", group.name ?? "");
    link.href = buildGroupURL(group.name);
    link.dataset.route = "/group";
    chip.append(link);
    return chip;
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
    userInput.disabled = busy;
    userSubmit.disabled = busy;
}

function isSearchCurrent(token) {
    return token === searchToken;
}

// Shows the combined Groups page's search state for the given term. The term
// also arrives from loadGroup when the exact by-name lookup misses.
async function performGroupSearch(term) {
    const token = ++searchGroupsToken;
    searchGroupsController?.abort();
    const controller = new AbortController();
    searchGroupsController = controller;

    const name = String(term ?? "").trim();
    if (!name) {
        clearGroupSearchResults();
        setGroupSearchStatus("Enter a group name.", "error");
        groupInput.focus();
        return;
    }

    setGroupSearchBusy(true);
    setGroupSearchStatus(`Searching for “${name}”…`);

    try {
        const response = await apiFetch(
            `/api/groups/search?name=${encodeURIComponent(name)}`,
            {
                headers: { Accept: "application/json" },
                signal: controller.signal,
            },
        );
        if (!isGroupSearchCurrent(token)) {
            return;
        }
        if (!response.ok) {
            throw await readError(response);
        }

        const body = await response.json();
        if (!isGroupSearchCurrent(token)) {
            return;
        }

        searchGroups = Array.isArray(body.groups) ? body.groups : [];
        resetGroupSearchControls();

        if (searchGroups.length === 0) {
            clearGroupSearchResults();
            setGroupSearchStatus(`No groups match “${name}”.`);
            return;
        }

        setGroupSearchStatus("");
        renderGroupSearchResults();
    } catch (error) {
        if (isAbort(error) || !isGroupSearchCurrent(token)) {
            return;
        }
        clearGroupSearchResults();
        setGroupSearchStatus(errorMessage(error), "error");
    } finally {
        if (isGroupSearchCurrent(token)) {
            setGroupSearchBusy(false);
            searchGroupsController = null;
        }
    }
}

function clearGroupSearchResults() {
    searchGroups = [];
    searchGroupsResults.replaceChildren();
    searchGroupsResultsPanel.hidden = true;
    searchGroupsControls.hidden = true;
    searchGroupsToolbar.hidden = true;
    searchGroupsCount.textContent = "";
    searchGroupsCount.hidden = true;
    searchGroupsPagination.replaceChildren();
}

function renderGroupSearchResults() {
    if (searchGroups.length === 0) {
        clearGroupSearchResults();
        return;
    }

    const visible = sortGroups(
        searchGroups,
        searchGroupsSort.value,
        searchGroupsDirection.value,
    );
    searchGroupsPageSize.value = String(searchGroupsRowsPerPage);
    const totalPages = totalPageCount(visible.length, searchGroupsRowsPerPage);
    searchGroupsPage = clampPage(searchGroupsPage, totalPages);
    const start = (searchGroupsPage - 1) * searchGroupsRowsPerPage;
    const pageGroups = visible.slice(start, start + searchGroupsRowsPerPage);

    searchGroupsResults.replaceChildren(
        ...pageGroups.flatMap((group, index) => buildGroupRows(group, index)),
    );
    searchGroupsControls.hidden = false;
    searchGroupsToolbar.hidden = false;
    searchGroupsResultsPanel.hidden = false;
    searchGroupsCount.hidden = false;
    searchGroupsCount.textContent = `Page ${searchGroupsPage} of ${totalPages} · ${formatCount(
        visible.length,
    )} ${visible.length === 1 ? "group" : "groups"}`;
    renderPagination(
        searchGroupsPagination,
        searchGroupsPage,
        totalPages,
        goToSearchGroupsPage,
    );
}

function goToSearchGroupsPage(page) {
    const totalPages = totalPageCount(
        sortGroups(
            searchGroups,
            searchGroupsSort.value,
            searchGroupsDirection.value,
        ).length,
        searchGroupsRowsPerPage,
    );
    if (
        !Number.isInteger(page) ||
        page < 1 ||
        page > totalPages ||
        page === searchGroupsPage
    ) {
        return;
    }
    searchGroupsPage = page;
    renderGroupSearchResults();
}

// Returns a sorted copy. Ties fall back to name ascending, then group ID.
function sortGroups(groups, sort, direction) {
    const multiplier = direction === "desc" ? -1 : 1;

    return [...groups].sort((left, right) => {
        const primary =
            sort === "members"
                ? countValue(left.memberCount) - countValue(right.memberCount)
                : compareSearchNames(left, right);
        if (primary !== 0) {
            return primary * multiplier;
        }
        const byName = compareSearchNames(left, right);
        if (byName !== 0) {
            return byName;
        }
        return countValue(left.groupId) - countValue(right.groupId);
    });
}

function resetGroupSearchControls() {
    searchGroupsSort.value = "name";
    searchGroupsDirection.value = GROUP_SORT_DEFAULT_DIRECTIONS.name;
    searchGroupsPage = 1;
}

// The status rule for a group, shared by every page that shows one.
function groupStatus(group) {
    if (countValue(group.memberCount) === 0) {
        return { label: "Empty", tone: "none" };
    }
    if (countValue(group.unavailableMemberCount) > 0) {
        return { label: "Unavailable", tone: "failed" };
    }
    return { label: "Available", tone: "succeeded" };
}

function buildGroupURL(name) {
    return `/group?name=${encodeURIComponent(name ?? "")}`;
}

function buildGroupRows(group, index) {
    const row = document.createElement("tr");
    row.className = "app-row";

    const nameCell = newElement("td", "app-cell");
    const body = newElement("div", "app-cell__body");
    const info = newElement("div", "app-cell__info");
    const nameLink = newElement("a", "app-cell__name", group.name ?? "");
    nameLink.href = buildGroupURL(group.name);
    nameLink.dataset.route = "/group";
    info.append(
        nameLink,
        newElement(
            "p",
            "app-cell__meta",
            `Members: ${formatCount(group.memberCount)}`,
        ),
    );
    body.append(info);
    nameCell.append(body);

    const status = groupStatus(group);
    const statusCell = newElement("td");
    statusCell.append(badge(status.label, status.tone));

    const detailsID = `search-group-details-${group.groupId ?? "unknown"}-${index}`;
    const detailsRow = document.createElement("tr");
    detailsRow.className = "app-details-row";
    detailsRow.hidden = true;

    const detailsCell = newElement("td", "app-details-cell");
    detailsCell.colSpan = 3;

    const details = newElement("div", "app-details");
    details.id = detailsID;
    const list = newElement("dl", "details app-details__list");
    list.append(
        detailRow("Group name", group.name ?? ""),
        detailRow("Group ID", String(group.groupId ?? "")),
        detailRow("Member count", formatCount(group.memberCount)),
    );
    details.append(list);
    detailsCell.append(details);
    detailsRow.append(detailsCell);

    const expandCell = buildExpandToggle(
        group.name ?? "group",
        detailsID,
        detailsRow,
    );

    row.append(nameCell, statusCell, expandCell);
    return [row, detailsRow];
}

function setGroupSearchStatus(message, state) {
    searchGroupsStatus.textContent = message;
    if (state) {
        searchGroupsStatus.dataset.state = state;
    } else {
        delete searchGroupsStatus.dataset.state;
    }
}

function setGroupSearchBusy(busy) {
    searchGroupsBusy = busy;
    groupInput.disabled = busy;
    groupSubmit.disabled = busy;
}

function isGroupSearchCurrent(token) {
    return token === searchGroupsToken;
}

function enterGroup() {
    const parsed = parseGroupQuery(location.search);

    groupState.name = parsed.name;
    groupState.data = null;

    groupController?.abort();
    groupController = null;
    groupToken += 1;
    searchGroupsController?.abort();
    searchGroupsController = null;
    searchGroupsToken += 1;
    if (searchGroupsBusy) {
        setGroupSearchBusy(false);
    }

    groupInput.value = parsed.name;
    resetGroupMemberControls();
    resetGroupManagement();
    setGroupLoading(false);
    clearGroupPage();

    if (!parsed.name) {
        setGroupStatus("");
        return;
    }

    syncGroupURL();
    void loadGroup();
}

function leaveGroup() {
    groupController?.abort();
    groupController = null;
    groupToken += 1;
    groupAddController?.abort();
    groupAddController = null;
    groupAddToken += 1;
    // The combined page may also have a Search Groups request in flight.
    searchGroupsController?.abort();
    searchGroupsToken += 1;
    if (searchGroupsBusy) {
        setGroupSearchBusy(false);
        setGroupSearchStatus("");
    }
}

function parseGroupQuery(search) {
    const params = new URLSearchParams(search);
    return { name: (params.get("name") ?? "").trim() };
}

function pushGroupURL(name) {
    const url = buildGroupURL(name);
    if (url !== location.pathname + location.search + location.hash) {
        history.pushState({}, "", url);
        currentHref = url;
    }

    if (name !== groupState.name) {
        // A new term: hide the previous view or search results until the
        // combined lookup decides which mode to render.
        clearGroupPage();
    }

    groupState.name = name;
    groupInput.value = name;
    resetGroupMemberControls();
    resetGroupManagement();
    void loadGroup();
}

// Rewrites the URL in place, e.g. to the stored casing of the group name.
function syncGroupURL() {
    if (!groupState.name) {
        return;
    }

    const url = buildGroupURL(groupState.name);
    if (url !== location.pathname + location.search + location.hash) {
        history.replaceState({}, "", url);
        currentHref = url;
    }
}

async function loadGroup() {
    const token = ++groupToken;
    groupController?.abort();
    const controller = new AbortController();
    groupController = controller;

    if (!groupState.name) {
        setGroupLoading(false);
        clearGroupView();
        setGroupStatus("");
        groupController = null;
        return;
    }

    // Pin the term for this request so a late 404 can search for exactly
    // what was looked up.
    const name = groupState.name;

    setGroupLoading(true);
    setGroupStatus(`Loading “${name}”…`);

    try {
        const response = await apiFetch(
            `/api/groups/by-name?name=${encodeURIComponent(name)}`,
            {
                headers: { Accept: "application/json" },
                signal: controller.signal,
            },
        );
        if (!isGroupCurrent(token)) {
            return;
        }

        if (response.status === 404) {
            // Not an exact match: show search results for the same term.
            clearGroupView();
            setGroupStatus("");
            setGroupLoading(false);
            void performGroupSearch(name);
            return;
        }
        if (!response.ok) {
            throw await readError(response);
        }

        const body = await response.json();
        if (!isGroupCurrent(token)) {
            return;
        }

        applyGroupResponse(body);
        setGroupStatus("");
    } catch (error) {
        if (isAbort(error) || !isGroupCurrent(token)) {
            return;
        }
        setGroupStatus(errorMessage(error), "error");
    } finally {
        if (isGroupCurrent(token)) {
            setGroupLoading(false);
            groupController = null;
        }
    }
}

function applyGroupResponse(body) {
    const group = body?.group;
    if (!group) {
        throw new Error("The server did not return a group.");
    }

    // View mode: never leave search results showing alongside the group.
    clearGroupSearchResults();
    setGroupSearchStatus("");

    groupState.data = {
        group,
        members: Array.isArray(body.members) ? body.members : [],
    };

    if (typeof group.name === "string" && group.name !== "") {
        groupState.name = group.name;
        groupInput.value = group.name;
    }

    renderGroupSummary(group);
    renderGroupMembers();
    groupAddMembers.hidden = false;
    renderGroupAddResults();
    syncGroupURL();
}

function renderGroupSummary(group) {
    groupSummary.hidden = false;
    const status = groupStatus(group);
    groupStatusBadge.textContent = status.label;
    groupStatusBadge.dataset.status = status.tone;

    groupSummaryDetails.replaceChildren(
        detailRow("Group name", group.name ?? ""),
        detailRow("Group ID", String(group.groupId ?? "")),
        detailRow("Members", formatCount(group.memberCount ?? 0)),
    );
}

// Re-renders the member table from groupState without refetching, so sort
// changes are instant.
function renderGroupMembers() {
    const data = groupState.data;
    if (!data) {
        return;
    }

    groupMembers.hidden = false;
    const members = data.members;
    if (members.length === 0) {
        groupMembersControls.hidden = true;
        groupMembersPanel.hidden = true;
        groupMemberRows.replaceChildren();
        groupMembersCount.textContent = "No members yet.";
        return;
    }

    groupMembersControls.hidden = false;
    groupMembersPanel.hidden = false;
    groupMembersCount.textContent =
        members.length === 1
            ? "1 member"
            : `${formatCount(members.length)} members`;

    const sorted = sortSteamUsers(
        members,
        groupMembersSort.value,
        groupMembersDirection.value,
    );
    groupMemberRows.replaceChildren(
        ...sorted.flatMap((user, index) =>
            buildSteamUserRows(user, index, {
                idPrefix: "group-member-details",
                actionCell: buildGroupMemberActionCell,
            }),
        ),
    );
}

function buildGroupMemberActionCell(user) {
    const cell = newElement("td");
    const button = newElement("button", "button--secondary", "Remove");
    button.type = "button";
    button.dataset.groupRemoveSteamId = String(user.steamId ?? "");
    button.disabled = groupMutationBusy;
    button.setAttribute(
        "aria-label",
        `Remove ${user.name ?? "user"} from ${groupState.data?.group?.name ?? "group"}`,
    );
    cell.append(button);
    return cell;
}

function resetGroupMemberControls() {
    groupMembersSort.value = "name";
    groupMembersDirection.value = SEARCH_SORT_DEFAULT_DIRECTIONS.name;
}

function clearGroupView() {
    groupState.data = null;
    groupSummary.hidden = true;
    groupMembers.hidden = true;
    groupAddMembers.hidden = true;
    groupSummaryDetails.replaceChildren();
    groupMembersCount.textContent = "";
    groupMemberRows.replaceChildren();
}

function setGroupLoading(loading) {
    groupSummary.setAttribute("aria-busy", String(loading));
    groupMembers.setAttribute("aria-busy", String(loading));
}

// The combined page shows either the group view or search results, never
// both. This hides both states while a new lookup is pending.
function clearGroupPage() {
    clearGroupView();
    clearGroupSearchResults();
    setGroupStatus("");
    setGroupSearchStatus("");
}

function setGroupStatus(message, state) {
    groupStatusLine.replaceChildren();
    groupStatusLine.textContent = message;
    if (state) {
        groupStatusLine.dataset.state = state;
    } else {
        delete groupStatusLine.dataset.state;
    }
}

function isGroupCurrent(token) {
    return token === groupToken;
}

// Clears per-group management UI: the rename form and the add-member search.
function resetGroupManagement() {
    setRenameFormOpen(false);
    groupAddController?.abort();
    groupAddController = null;
    groupAddToken += 1;
    groupAddUsers = [];
    groupAddInput.value = "";
    groupAddInput.disabled = false;
    groupAddButton.disabled = false;
    groupAddResults.replaceChildren();
    setGroupFormStatus(groupAddStatus, "");
}

function setGroupFormStatus(element, message, state) {
    element.textContent = message;
    if (state) {
        element.dataset.state = state;
    } else {
        delete element.dataset.state;
    }
}

function setGroupMutationBusy(busy) {
    groupMutationBusy = busy;
    createGroupButton.disabled = busy;
    groupCreateButton.disabled = busy;
    groupCreateCancel.disabled = busy;
    groupRenameButton.disabled = busy;
    groupDeleteButton.disabled = busy;
    groupRenameSave.disabled = busy;
    for (const button of groupMemberRows.querySelectorAll(
        "button[data-group-remove-steam-id]",
    )) {
        button.disabled = busy;
    }
    // Add buttons also depend on membership, so rebuild them.
    renderGroupAddResults();
}

function currentGroup() {
    return groupState.data?.group ?? null;
}

function openCreateGroupDialog() {
    groupCreateInput.value = "";
    setGroupFormStatus(groupCreateStatus, "");
    createGroupDialog.showModal();
    groupCreateInput.focus();
}

async function createGroup() {
    if (groupMutationBusy) {
        return;
    }
    const name = groupCreateInput.value.trim();
    if (!name) {
        setGroupFormStatus(groupCreateStatus, "Enter a group name.", "error");
        groupCreateInput.focus();
        return;
    }

    setGroupMutationBusy(true);
    setGroupFormStatus(groupCreateStatus, `Creating “${name}”…`);
    try {
        const response = await apiFetch("/api/groups", {
            method: "POST",
            headers: {
                Accept: "application/json",
                "Content-Type": "application/json",
            },
            body: JSON.stringify({ name }),
        });
        if (!response.ok) {
            throw await readError(response);
        }
        const body = await response.json();
        const createdName = body?.group?.name ?? name;

        createGroupDialog.close();
        groupCreateInput.value = "";
        setGroupFormStatus(groupCreateStatus, "");
        pushGroupURL(createdName);
    } catch (error) {
        setGroupFormStatus(groupCreateStatus, errorMessage(error), "error");
    } finally {
        setGroupMutationBusy(false);
    }
}

function setRenameFormOpen(open) {
    groupRenameForm.hidden = !open;
    groupRenameButton.setAttribute("aria-expanded", String(open));
    setGroupFormStatus(groupRenameStatus, "");
    if (open) {
        groupRenameInput.value = currentGroup()?.name ?? "";
        groupRenameInput.focus();
        groupRenameInput.select();
    }
}

async function renameGroup() {
    const group = currentGroup();
    if (groupMutationBusy || !group) {
        return;
    }
    const name = groupRenameInput.value.trim();
    if (!name) {
        setGroupFormStatus(groupRenameStatus, "Enter a group name.", "error");
        groupRenameInput.focus();
        return;
    }

    setGroupMutationBusy(true);
    setGroupFormStatus(groupRenameStatus, "Saving…");
    try {
        const response = await apiFetch(
            `/api/groups/${encodeURIComponent(group.groupId)}`,
            {
                method: "PATCH",
                headers: {
                    Accept: "application/json",
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({ name }),
            },
        );
        if (!response.ok) {
            throw await readError(response);
        }
        const body = await response.json();
        const renamed = body?.group;
        if (!renamed || !groupState.data) {
            throw new Error("The server did not return a group.");
        }

        groupState.data.group = renamed;
        groupState.name = renamed.name;
        groupInput.value = renamed.name;
        syncGroupURL();
        renderGroupSummary(renamed);
        setRenameFormOpen(false);
        setGroupStatus(`Renamed group to “${renamed.name}”.`);
        groupRenameButton.focus();
    } catch (error) {
        setGroupFormStatus(groupRenameStatus, errorMessage(error), "error");
    } finally {
        setGroupMutationBusy(false);
        // The member Remove labels include the group name.
        renderGroupMembers();
    }
}

async function deleteGroup() {
    const group = currentGroup();
    if (groupMutationBusy || !group) {
        return;
    }
    if (!confirm(`Delete group “${group.name}”? This can't be undone.`)) {
        return;
    }

    setGroupMutationBusy(true);
    const deletedName = group.name ?? groupState.name;
    setGroupStatus(`Deleting “${deletedName}”…`);
    try {
        const response = await apiFetch(
            `/api/groups/${encodeURIComponent(group.groupId)}`,
            { method: "DELETE" },
        );
        if (!response.ok) {
            throw await readError(response);
        }

        // Stay on /group?name=<term>: the exact match is gone, so the
        // combined lookup lands in search mode and lists what remains.
        clearGroupView();
        setGroupStatus("");
        await performGroupSearch(deletedName);
        // Earlier results may still list the deleted group.
        setGroupSearchStatus(`Deleted group “${deletedName}”.`);
    } catch (error) {
        setGroupStatus(errorMessage(error), "error");
    } finally {
        setGroupMutationBusy(false);
    }
}

async function performGroupAddSearch() {
    const token = ++groupAddToken;
    groupAddController?.abort();
    const controller = new AbortController();
    groupAddController = controller;

    const name = groupAddInput.value.trim();
    if (!name) {
        groupAddUsers = [];
        renderGroupAddResults();
        setGroupFormStatus(groupAddStatus, "Enter a Steam name.", "error");
        groupAddInput.focus();
        return;
    }

    groupAddInput.disabled = true;
    groupAddButton.disabled = true;
    setGroupFormStatus(groupAddStatus, `Searching for “${name}”…`);

    try {
        const response = await apiFetch(
            `/api/steamusers/search?name=${encodeURIComponent(name)}`,
            {
                headers: { Accept: "application/json" },
                signal: controller.signal,
            },
        );
        if (token !== groupAddToken) {
            return;
        }
        if (!response.ok) {
            throw await readError(response);
        }
        const body = await response.json();
        if (token !== groupAddToken) {
            return;
        }

        groupAddUsers = Array.isArray(body.steamUsers) ? body.steamUsers : [];
        renderGroupAddResults();
        setGroupFormStatus(
            groupAddStatus,
            groupAddUsers.length === 0 ? `No users match “${name}”.` : "",
        );
    } catch (error) {
        if (isAbort(error) || token !== groupAddToken) {
            return;
        }
        groupAddUsers = [];
        renderGroupAddResults();
        setGroupFormStatus(groupAddStatus, errorMessage(error), "error");
    } finally {
        if (token === groupAddToken) {
            groupAddInput.disabled = false;
            groupAddButton.disabled = false;
            groupAddController = null;
        }
    }
}

function groupMemberIDs() {
    return new Set(
        (groupState.data?.members ?? []).map((member) =>
            String(member.steamId ?? ""),
        ),
    );
}

function renderGroupAddResults() {
    const memberIDs = groupMemberIDs();
    groupAddResults.replaceChildren(
        ...groupAddUsers.map((user) => buildGroupAddResult(user, memberIDs)),
    );
}

function buildGroupAddResult(user, memberIDs) {
    const item = newElement("li", "result");
    const main = newElement("div", "result__main");
    const status = importStatus(user.latestImportStatus ?? null);

    const heading = newElement("div", "result__heading");
    heading.append(
        newElement("p", "result__name", user.name ?? ""),
        badge(status.label, status.tone),
    );
    main.append(
        heading,
        newElement("p", "result__id", `Steam ID ${user.steamId ?? ""}`),
    );

    const steamID = String(user.steamId ?? "");
    const member = memberIDs.has(steamID);
    const action = newElement(
        "button",
        "result__link",
        member ? "Added" : "Add",
    );
    action.type = "button";
    action.dataset.groupAddSteamId = steamID;
    action.disabled = member || groupMutationBusy;
    if (!member) {
        action.setAttribute(
            "aria-label",
            `Add ${user.name ?? "user"} to ${currentGroup()?.name ?? "group"}`,
        );
    }

    item.append(main, action);
    return item;
}

async function addGroupMember(steamID) {
    const group = currentGroup();
    const user = groupAddUsers.find(
        (candidate) => String(candidate.steamId ?? "") === steamID,
    );
    if (groupMutationBusy || !group || !user || groupMemberIDs().has(steamID)) {
        return;
    }

    setGroupMutationBusy(true);
    setGroupFormStatus(groupAddStatus, `Adding “${user.name}”…`);
    try {
        const response = await apiFetch(
            `/api/groups/${encodeURIComponent(group.groupId)}/members/${encodeURIComponent(steamID)}`,
            { method: "PUT" },
        );
        if (!response.ok) {
            throw await readError(response);
        }
        await loadGroup();
        setGroupFormStatus(
            groupAddStatus,
            `Added “${user.name}” to “${currentGroup()?.name ?? group.name}”.`,
        );
    } catch (error) {
        setGroupFormStatus(groupAddStatus, errorMessage(error), "error");
    } finally {
        setGroupMutationBusy(false);
    }
}

async function removeGroupMember(steamID) {
    const group = currentGroup();
    const member = (groupState.data?.members ?? []).find(
        (candidate) => String(candidate.steamId ?? "") === steamID,
    );
    if (groupMutationBusy || !group || !member) {
        return;
    }

    setGroupMutationBusy(true);
    setGroupStatus(`Removing “${member.name}”…`);
    try {
        const response = await apiFetch(
            `/api/groups/${encodeURIComponent(group.groupId)}/members/${encodeURIComponent(steamID)}`,
            { method: "DELETE" },
        );
        if (!response.ok) {
            throw await readError(response);
        }
        await loadGroup();
        setGroupStatus(
            `Removed “${member.name}” from “${currentGroup()?.name ?? group.name}”.`,
        );
    } catch (error) {
        setGroupStatus(errorMessage(error), "error");
    } finally {
        setGroupMutationBusy(false);
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
    searchController?.abort();
    searchController = null;
    searchToken += 1;
    if (searchBusy) {
        setSearchBusy(false);
    }

    userInput.value = parsed.name;
    userPageSize.value = String(parsed.pageSize);
    setActiveTab(parsed.list);
    setUserLoading(false);
    clearUserPage();
    resetUserGroupSearch();

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
    resetUserGroupSearch();
    // The combined page may also have a Search Users request in flight.
    searchController?.abort();
    searchToken += 1;
    if (searchBusy) {
        setSearchBusy(false);
        setSearchStatus("");
    }
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

function searchPageSizeFromValue(value) {
    const number = Number(value);
    return SEARCH_PAGE_SIZES.includes(number) ? number : 10;
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

    if (name !== userState.name) {
        // A new term: hide the previous view or search results until the
        // combined lookup decides which mode to render.
        clearUserPage();
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

    // Pin the term for this request so a late 404 can search for exactly
    // what was looked up.
    const name = userState.name;

    setUserLoading(true);
    setUserStatus(`Loading “${name}”…`);

    const params = new URLSearchParams({
        name,
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
            // Not an exact match: fall through to search results for the
            // same term.
            setUserLoading(false);
            clearUserView();
            setUserStatus("");
            void performSearch(name);
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

    // View mode: never leave search results showing alongside the user.
    clearSearchResults();
    setSearchStatus("");

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
    renderUserGroups(body);
    renderUserApps(body);
    syncUserURL();
}

function userGroupIDs() {
    const groups = Array.isArray(userState.data?.groups)
        ? userState.data.groups
        : [];
    return new Set(groups.map((group) => String(group.groupId ?? "")));
}

function renderUserGroups(data) {
    const groups = Array.isArray(data?.groups) ? data.groups : [];
    userGroups.hidden = false;
    userGroupsEmpty.hidden = groups.length > 0;
    userGroupsList.replaceChildren(...groups.map(buildUserGroupItem));
    renderUserGroupSearchResults();
}

function buildUserGroupItem(group) {
    const userName = userState.data?.steamUser?.name ?? "user";
    const item = newElement("li", "selected-user");
    const main = newElement("div", "selected-user__main");
    const link = newElement("a", "selected-user__name", group.name ?? "");
    link.href = buildGroupURL(group.name);
    link.dataset.route = "/group";
    main.append(link);

    const remove = newElement(
        "button",
        "button--secondary selected-user__remove",
        "Remove",
    );
    remove.type = "button";
    remove.dataset.userGroupRemoveId = String(group.groupId ?? "");
    remove.disabled = userGroupsBusy;
    remove.setAttribute(
        "aria-label",
        `Remove ${userName} from ${group.name ?? "group"}`,
    );
    item.append(main, remove);
    return item;
}

function resetUserGroupSearch() {
    userGroupSearchController?.abort();
    userGroupSearchController = null;
    userGroupSearchToken += 1;
    userGroupSearchResults = [];
    userGroupAddInput.value = "";
    userGroupAddInput.disabled = false;
    userGroupAddButton.disabled = false;
    userGroupAddResults.replaceChildren();
    setGroupFormStatus(userGroupsStatus, "");
}

async function performUserGroupSearch() {
    const token = ++userGroupSearchToken;
    userGroupSearchController?.abort();
    const controller = new AbortController();
    userGroupSearchController = controller;

    const name = userGroupAddInput.value.trim();
    if (!name) {
        userGroupSearchResults = [];
        renderUserGroupSearchResults();
        setGroupFormStatus(userGroupsStatus, "Enter a group name.", "error");
        userGroupAddInput.focus();
        return;
    }

    userGroupAddInput.disabled = true;
    userGroupAddButton.disabled = true;
    setGroupFormStatus(userGroupsStatus, `Searching for “${name}”…`);

    try {
        const response = await apiFetch(
            `/api/groups/search?name=${encodeURIComponent(name)}`,
            {
                headers: { Accept: "application/json" },
                signal: controller.signal,
            },
        );
        if (token !== userGroupSearchToken) {
            return;
        }
        if (!response.ok) {
            throw await readError(response);
        }
        const body = await response.json();
        if (token !== userGroupSearchToken) {
            return;
        }

        userGroupSearchResults = Array.isArray(body.groups) ? body.groups : [];
        renderUserGroupSearchResults();
        setGroupFormStatus(
            userGroupsStatus,
            userGroupSearchResults.length === 0
                ? `No groups match “${name}”.`
                : "",
        );
    } catch (error) {
        if (isAbort(error) || token !== userGroupSearchToken) {
            return;
        }
        userGroupSearchResults = [];
        renderUserGroupSearchResults();
        setGroupFormStatus(userGroupsStatus, errorMessage(error), "error");
    } finally {
        if (token === userGroupSearchToken) {
            userGroupAddInput.disabled = false;
            userGroupAddButton.disabled = false;
            userGroupSearchController = null;
        }
    }
}

function renderUserGroupSearchResults() {
    const memberOf = userGroupIDs();
    userGroupAddResults.replaceChildren(
        ...userGroupSearchResults.map((group) =>
            buildUserGroupSearchResult(group, memberOf),
        ),
    );
}

// Member counts and availability aren't shown: they'd go stale as soon as
// this page adds or removes the user.
function buildUserGroupSearchResult(group, memberOf) {
    const item = newElement("li", "result");
    const main = newElement("div", "result__main");
    const heading = newElement("div", "result__heading");
    heading.append(newElement("p", "result__name", group.name ?? ""));
    main.append(heading);

    const groupID = String(group.groupId ?? "");
    const member = memberOf.has(groupID);
    const action = newElement(
        "button",
        "result__link",
        member ? "Added" : "Add",
    );
    action.type = "button";
    action.dataset.userGroupAddId = groupID;
    action.disabled = member || userGroupsBusy;
    if (!member) {
        action.setAttribute(
            "aria-label",
            `Add ${userState.data?.steamUser?.name ?? "user"} to ${
                group.name ?? "group"
            }`,
        );
    }

    item.append(main, action);
    return item;
}

function setUserGroupsBusy(busy) {
    userGroupsBusy = busy;
    for (const button of userGroupsList.querySelectorAll(
        "button[data-user-group-remove-id]",
    )) {
        button.disabled = busy;
    }
    renderUserGroupSearchResults();
}

async function addUserToGroup(groupID) {
    const user = userState.data?.steamUser;
    const group = userGroupSearchResults.find(
        (candidate) => String(candidate.groupId ?? "") === groupID,
    );
    if (userGroupsBusy || !user || !group || userGroupIDs().has(groupID)) {
        return;
    }

    await mutateUserGroup(
        "PUT",
        group,
        user,
        `Adding “${user.name}” to “${group.name}”…`,
        `Added “${user.name}” to “${group.name}”.`,
    );
}

async function removeUserFromGroup(groupID) {
    const user = userState.data?.steamUser;
    const group = (userState.data?.groups ?? []).find(
        (candidate) => String(candidate.groupId ?? "") === groupID,
    );
    if (userGroupsBusy || !user || !group) {
        return;
    }

    await mutateUserGroup(
        "DELETE",
        group,
        user,
        `Removing “${user.name}” from “${group.name}”…`,
        `Removed “${user.name}” from “${group.name}”.`,
    );
}

// Applies a membership change, then reloads the user view. The Apps tab,
// page and page size live in the URL, so the reload keeps them.
async function mutateUserGroup(
    method,
    group,
    user,
    pendingMessage,
    doneMessage,
) {
    setUserGroupsBusy(true);
    setGroupFormStatus(userGroupsStatus, pendingMessage);
    try {
        const response = await apiFetch(
            `/api/groups/${encodeURIComponent(group.groupId)}/members/${encodeURIComponent(user.steamId)}`,
            { method },
        );
        if (!response.ok) {
            throw await readError(response);
        }
        await loadUser();
        // Another user may have been opened while this was in flight.
        if (
            String(userState.data?.steamUser?.steamId ?? "") ===
            String(user.steamId)
        ) {
            setGroupFormStatus(userGroupsStatus, doneMessage);
        } else {
            setGroupFormStatus(userGroupsStatus, "");
        }
    } catch (error) {
        setGroupFormStatus(userGroupsStatus, errorMessage(error), "error");
    } finally {
        setUserGroupsBusy(false);
    }
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
        const avatarRow = newElement(
            "div",
            "details__row details__row--avatar",
        );
        avatarRow.append(
            newElement("dt", "visually-hidden", "Avatar"),
            avatarValue,
        );
        rows.push(avatarRow);
    }

    rows.push(
        detailRowNode("Steam name", steamNameNode(user)),
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

// steamNameNode shows the stored handle, with a quiet badge for users whose
// name is a custom label rather than a real Steam vanity.
function steamNameNode(user) {
    const node = newElement("span");
    node.append(user.name ?? "");
    if (user.customUsername) {
        node.append(document.createTextNode(" "), badge("Custom name", "none"));
    }
    return node;
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
    const current = parsePositiveInteger(data.page, 1);
    const totalPages = countValue(data.totalPages);
    renderPagination(userPagination, current, totalPages, goToUserPage);
}

function renderPagination(container, current, totalPages, goToPage) {
    container.replaceChildren();

    const hasPages = totalPages > 0;
    container.append(
        paginationButton(
            "Previous",
            current - 1,
            current <= 1 || !hasPages,
            "Previous page",
            goToPage,
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
                container.append(ellipsis);
                continue;
            }

            const button = paginationButton(
                String(item),
                item,
                false,
                `Page ${item}`,
                goToPage,
            );
            if (item === current) {
                button.setAttribute("aria-current", "page");
                button.disabled = true;
            }
            container.append(button);
        }
    }

    container.append(
        paginationButton(
            "Next",
            current + 1,
            !hasPages || current >= totalPages,
            "Next page",
            goToPage,
        ),
    );
}

function paginationButton(text, page, disabled, label, goToPage) {
    const button = newElement("button", "pagination__button", text);
    button.type = "button";
    button.disabled = disabled;
    button.setAttribute("aria-label", label);
    button.addEventListener("click", () => goToPage(page));
    return button;
}

function totalPageCount(itemCount, pageSize) {
    return itemCount === 0 ? 0 : Math.ceil(itemCount / pageSize);
}

function clampPage(page, totalPages) {
    if (totalPages === 0) {
        return 1;
    }
    return Math.min(Math.max(page, 1), totalPages);
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
    userGroups.hidden = true;
    userGroupsList.replaceChildren();
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
    userGroups.setAttribute("aria-busy", String(loading));
    userApps.setAttribute("aria-busy", String(loading));
}

// The combined page shows either the user view or search results, never
// both. This hides both states while a new lookup is pending.
function clearUserPage() {
    clearUserView();
    clearSearchResults();
    setUserStatus("");
    setSearchStatus("");
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

    commonGamesState.selectedUsers.clear();
    commonGamesState.selectedGroups.clear();
    commonGamesState.results = null;
    commonGamesState.submitted = parsed.submitted;
    commonGamesLimit.value = String(parsed.limit ?? COMMON_GAMES_LIMIT_DEFAULT);

    clearCommonGamesSearchResults();
    clearCommonGamesResultsView();
    renderCommonGamesSelection();
    setCommonGamesStatus("");

    if (parsed.steamIDs.length === 0 && parsed.groupIDs.length === 0) {
        commonGamesState.submitted = false;
        return;
    }
    // Individually listed users alone can't exceed the limit in a valid link,
    // and the by-ids endpoint would reject them anyway.
    if (parsed.steamIDs.length > MAX_COMMON_GAME_USERS) {
        commonGamesState.submitted = false;
        setCommonGamesStatus(
            `This shared link now includes more than ${MAX_COMMON_GAME_USERS} users.`,
            "error",
        );
        return;
    }

    void loadCommonGamesSelectionFromURL(
        parsed.steamIDs,
        parsed.groupIDs,
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
    return {
        steamIDs: parseCommonGamesIDList(params.get("steamIds")),
        groupIDs: parseCommonGamesIDList(params.get("groupIds")),
        limit: commonGamesLimitFromValue(params.get("limit")),
        submitted: params.get("submitted") === "1",
    };
}

// Parses a comma-separated list of positive integer IDs, dropping invalid
// entries, leading zeros and duplicates. IDs stay strings.
function parseCommonGamesIDList(raw) {
    const seen = new Set();
    const ids = [];
    for (const rawID of (raw ?? "").split(",")) {
        const value = rawID.trim();
        if (!/^\d+$/.test(value)) {
            continue;
        }
        const id = value.replace(/^0+(?=\d)/, "");
        if (id === "0" || seen.has(id)) {
            continue;
        }
        seen.add(id);
        ids.push(id);
    }
    return ids;
}

function buildCommonGamesURL() {
    const params = new URLSearchParams();
    const steamIDs = [...commonGamesState.selectedUsers.keys()];
    if (steamIDs.length > 0) {
        params.set("steamIds", steamIDs.join(","));
    }
    const groupIDs = [...commonGamesState.selectedGroups.keys()];
    if (groupIDs.length > 0) {
        params.set("groupIds", groupIDs.join(","));
    }

    const limit = parseCommonGamesLimit();
    if (limit !== null) {
        params.set("limit", String(limit));
    }
    if (commonGamesState.submitted) {
        params.set("submitted", "1");
    }

    const query = params.toString();
    return query ? `/?${query}` : "/";
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

async function loadCommonGamesSelectionFromURL(steamIDs, groupIDs, submitted) {
    const token = ++commonGamesUsersToken;
    commonGamesUsersController?.abort();
    const controller = new AbortController();
    commonGamesUsersController = controller;

    setCommonGamesStatus(
        `Loading ${commonGamesSharedCountPhrase(
            steamIDs.length,
            groupIDs.length,
        )}…`,
    );

    const fetchJSON = async (path, notFoundMessage) => {
        const response = await apiFetch(path, {
            headers: { Accept: "application/json" },
            signal: controller.signal,
        });
        if (response.status === 404 && notFoundMessage) {
            throw new Error(notFoundMessage);
        }
        if (!response.ok) {
            throw await readError(response);
        }
        return response.json();
    };

    try {
        // Groups are live references: membership is fetched fresh, so a
        // link reflects members added or removed since it was shared.
        const [usersBody, groupsBody] = await Promise.all([
            steamIDs.length > 0
                ? fetchJSON(
                      `/api/steamusers/by-ids?steamIds=${encodeURIComponent(
                          steamIDs.join(","),
                      )}`,
                  )
                : null,
            groupIDs.length > 0
                ? fetchJSON(
                      `/api/groups/by-ids?groupIds=${encodeURIComponent(
                          groupIDs.join(","),
                      )}`,
                      "A shared group no longer exists.",
                  )
                : null,
        ]);
        if (!isCommonGamesUsersCurrent(token)) {
            return;
        }

        const users = Array.isArray(usersBody?.steamUsers)
            ? usersBody.steamUsers
            : [];
        const usersByID = new Map(
            users.map((user) => [String(user.steamId ?? ""), user]),
        );
        const groups = Array.isArray(groupsBody?.groups)
            ? groupsBody.groups
            : [];
        const groupsByID = new Map(
            groups.map((group) => [String(group.groupId ?? ""), group]),
        );

        commonGamesState.selectedUsers.clear();
        for (const steamID of steamIDs) {
            const user = usersByID.get(steamID);
            if (user?.hasSucceededImport === true) {
                commonGamesState.selectedUsers.set(steamID, user);
            }
        }
        commonGamesState.selectedGroups.clear();
        for (const groupID of groupIDs) {
            const group = groupsByID.get(groupID);
            if (group && countValue(group.unavailableMemberCount) === 0) {
                commonGamesState.selectedGroups.set(groupID, group);
            }
        }
        renderCommonGamesSelection();

        const distinctCount = commonGamesSelectedSteamIDs().size;
        if (distinctCount > MAX_COMMON_GAME_USERS) {
            setCommonGamesStatus(
                `This shared link now includes more than ${MAX_COMMON_GAME_USERS} users.`,
                "error",
            );
            return;
        }
        if (distinctCount < 2) {
            setCommonGamesStatus(
                "This shared link no longer includes enough imported users.",
                "error",
            );
            return;
        }

        const missingUsers =
            steamIDs.length - commonGamesState.selectedUsers.size;
        const missingGroups =
            groupIDs.length - commonGamesState.selectedGroups.size;
        if (missingUsers + missingGroups > 0) {
            setCommonGamesStatus(
                `${commonGamesSharedCountPhrase(missingUsers, missingGroups)} ${
                    missingUsers + missingGroups === 1 ? "is" : "are"
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

// "2 shared users", "1 shared group", "1 shared user and 2 shared groups".
function commonGamesSharedCountPhrase(userCount, groupCount) {
    const parts = [];
    if (userCount > 0) {
        parts.push(
            `${formatCount(userCount)} shared ${
                userCount === 1 ? "user" : "users"
            }`,
        );
    }
    if (groupCount > 0) {
        parts.push(
            `${formatCount(groupCount)} shared ${
                groupCount === 1 ? "group" : "groups"
            }`,
        );
    }
    return parts.join(" and ");
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
        setCommonGamesSearchStatus("Enter a user or group name.", "error");
        commonGamesSearchInput.focus();
        return;
    }

    const controller = new AbortController();
    commonGamesSearchController = controller;

    setCommonGamesSearchBusy(true);
    setCommonGamesSearchStatus(`Searching for “${name}”…`);

    const fetchJSON = async (path) => {
        const response = await apiFetch(path, {
            headers: { Accept: "application/json" },
            signal: controller.signal,
        });
        if (!response.ok) {
            throw await readError(response);
        }
        return response.json();
    };

    try {
        const query = encodeURIComponent(name);
        const [usersBody, groupsBody] = await Promise.all([
            fetchJSON(`/api/steamusers/search?name=${query}`),
            fetchJSON(`/api/groups/search?name=${query}`),
        ]);
        if (!isCommonGamesSearchCurrent(token)) {
            return;
        }

        commonGamesState.searchUsers = Array.isArray(usersBody?.steamUsers)
            ? usersBody.steamUsers
            : [];
        commonGamesState.searchGroups = Array.isArray(groupsBody?.groups)
            ? groupsBody.groups
            : [];

        if (
            commonGamesState.searchUsers.length === 0 &&
            commonGamesState.searchGroups.length === 0
        ) {
            clearCommonGamesSearchResults();
            setCommonGamesSearchStatus(`No users or groups match “${name}”.`);
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
    const { searchUsers, searchGroups } = commonGamesState;

    commonGamesSearchResultsPanel.hidden =
        searchUsers.length === 0 && searchGroups.length === 0;
    commonGamesUserResultsSection.hidden = searchUsers.length === 0;
    commonGamesGroupResultsSection.hidden = searchGroups.length === 0;
    commonGamesSearchResults.replaceChildren(
        ...searchUsers.map(buildCommonGamesSearchResult),
    );
    commonGamesGroupResults.replaceChildren(
        ...searchGroups.map(buildCommonGamesGroupResult),
    );
}

function clearCommonGamesSearchResults() {
    commonGamesState.searchUsers = [];
    commonGamesState.searchGroups = [];
    renderCommonGamesSearchResults();
}

// Distinct Steam IDs covered by the selection (individual users plus group
// members), optionally including extra IDs to preview an addition.
function commonGamesSelectedSteamIDs(extra = []) {
    const ids = new Set(commonGamesState.selectedUsers.keys());
    for (const group of commonGamesState.selectedGroups.values()) {
        for (const member of commonGamesGroupMembers(group)) {
            ids.add(String(member.steamId));
        }
    }
    for (const id of extra) {
        ids.add(String(id));
    }
    return ids;
}

function commonGamesGroupMembers(group) {
    return Array.isArray(group?.members) ? group.members : [];
}

function commonGamesGroupMemberIDs(group) {
    return commonGamesGroupMembers(group).map((member) =>
        String(member.steamId ?? ""),
    );
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
    // A user already covered by a selected group adds nobody, so it stays
    // addable even at the limit.
    const atLimit =
        commonGamesSelectedSteamIDs([steamID]).size > MAX_COMMON_GAME_USERS;
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

function commonGamesGroupStatus(group) {
    const status = groupStatus(group);
    if (status.label === "Available") {
        return { label: "Ready", tone: "succeeded" };
    }
    return status;
}

function buildCommonGamesGroupResult(group) {
    const item = newElement("li", "result");
    const main = newElement("div", "result__main");
    const status = commonGamesGroupStatus(group);

    const heading = newElement("div", "result__heading");
    heading.append(
        newElement("p", "result__name", group.name ?? ""),
        badge(status.label, status.tone),
    );
    main.append(heading);

    const groupID = String(group.groupId ?? "");
    const selected = commonGamesState.selectedGroups.has(groupID);
    const available = countValue(group.unavailableMemberCount) === 0;
    const currentSize = commonGamesSelectedSteamIDs().size;
    const newSize = commonGamesSelectedSteamIDs(
        commonGamesGroupMemberIDs(group),
    ).size;
    const overLimit = newSize > MAX_COMMON_GAME_USERS;
    const addable = !selected && available && !overLimit;

    let meta = `Members: ${formatCount(group.memberCount)}`;
    if (addable) {
        meta += ` · adds ${formatCount(newSize - currentSize)} new`;
    }
    main.append(newElement("p", "result__id", meta));

    const action = newElement("button", "result__link");
    action.type = "button";
    action.dataset.addGroupId = groupID;

    if (selected) {
        action.textContent = "Added";
        action.disabled = true;
    } else if (!available) {
        action.textContent = "Unavailable";
        action.disabled = true;
        action.title = "A member has no completed import";
    } else if (overLimit) {
        action.textContent = "Add";
        action.disabled = true;
        action.title = `Adding this group would exceed ${MAX_COMMON_GAME_USERS} users.`;
    } else {
        action.textContent = "Add";
    }

    item.append(main, action);
    return item;
}

function addCommonGamesUser(user) {
    const steamID = String(user.steamId ?? "");
    if (
        steamID === "" ||
        user.hasSucceededImport !== true ||
        commonGamesSelectedSteamIDs([steamID]).size > MAX_COMMON_GAME_USERS ||
        commonGamesState.selectedUsers.has(steamID)
    ) {
        return;
    }

    commonGamesState.selectedUsers.set(steamID, user);
    commonGamesSelectionChanged();
}

function removeCommonGamesUser(steamID) {
    if (!commonGamesState.selectedUsers.delete(String(steamID))) {
        return;
    }

    commonGamesSelectionChanged();
}

function addCommonGamesGroup(group) {
    const groupID = String(group.groupId ?? "");
    if (
        groupID === "" ||
        countValue(group.unavailableMemberCount) > 0 ||
        commonGamesState.selectedGroups.has(groupID) ||
        commonGamesSelectedSteamIDs(commonGamesGroupMemberIDs(group)).size >
            MAX_COMMON_GAME_USERS
    ) {
        return;
    }

    commonGamesState.selectedGroups.set(groupID, group);
    commonGamesSelectionChanged();
}

function removeCommonGamesGroup(groupID) {
    if (!commonGamesState.selectedGroups.delete(String(groupID))) {
        return;
    }

    commonGamesSelectionChanged();
}

function commonGamesSelectionChanged() {
    invalidateCommonGamesResults();
    renderCommonGamesSelection();
    renderCommonGamesSearchResults();
}

function renderCommonGamesSelection() {
    const users = [...commonGamesState.selectedUsers.values()];
    const groups = [...commonGamesState.selectedGroups.values()];
    const distinctCount = commonGamesSelectedSteamIDs().size;
    const atLimit = distinctCount >= MAX_COMMON_GAME_USERS;

    commonGamesSelectedCount.textContent = `${distinctCount} / ${MAX_COMMON_GAME_USERS}`;
    commonGamesSelectedEmpty.hidden = users.length > 0 || groups.length > 0;
    commonGamesSelectedHint.hidden = !atLimit;
    commonGamesSelectedHint.textContent = atLimit
        ? `Selection limit reached (${MAX_COMMON_GAME_USERS} users).`
        : "";
    commonGamesSelectedGroupsSection.hidden = groups.length === 0;
    commonGamesSelectedGroups.replaceChildren(
        ...groups.map(buildCommonGamesSelectedGroup),
    );
    commonGamesSelectedUsersSection.hidden = users.length === 0;
    commonGamesSelectedList.replaceChildren(
        ...users.map(buildCommonGamesSelectedUser),
    );

    syncCommonGamesControls();
}

function buildCommonGamesSelectedGroup(group) {
    const item = newElement("li", "selected-user");
    const main = newElement("div", "selected-user__main");
    const memberCount = commonGamesGroupMembers(group).length;
    main.append(
        newElement("span", "selected-user__name", group.name ?? ""),
        newElement(
            "span",
            "selected-user__id",
            `${formatCount(memberCount)} ${
                memberCount === 1 ? "member" : "members"
            }`,
        ),
    );

    const remove = newElement(
        "button",
        "button--secondary selected-user__remove",
        "Remove",
    );
    remove.type = "button";
    remove.dataset.removeGroupId = String(group.groupId ?? "");
    remove.setAttribute("aria-label", `Remove group ${group.name ?? ""}`);
    item.append(main, remove);

    return item;
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
    const enoughUsers = commonGamesSelectedSteamIDs().size >= 2;

    commonGamesSubmit.disabled = !enoughUsers || commonGamesBusy;
    commonGamesLimit.disabled = commonGamesBusy;
    commonGamesSubmitHint.hidden = enoughUsers;
    commonGamesSubmitHint.textContent = enoughUsers
        ? ""
        : "Select at least 2 distinct users to continue.";
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
    const usersByID = new Map();
    for (const user of commonGamesState.selectedUsers.values()) {
        const steamID = String(user.steamId ?? "");
        usersByID.set(steamID, { steamId: steamID, name: user.name ?? "" });
    }
    for (const group of commonGamesState.selectedGroups.values()) {
        for (const member of commonGamesGroupMembers(group)) {
            const steamID = String(member.steamId ?? "");
            if (!usersByID.has(steamID)) {
                usersByID.set(steamID, {
                    steamId: steamID,
                    name: member.name ?? "",
                });
            }
        }
    }

    if (usersByID.size < 2) {
        setCommonGamesStatus("Select at least 2 distinct users.", "error");
        return;
    }
    if (usersByID.size > MAX_COMMON_GAME_USERS) {
        setCommonGamesStatus(
            `Select at most ${MAX_COMMON_GAME_USERS} distinct users.`,
            "error",
        );
        return;
    }
    const users = [...usersByID.values()];

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
                steamIds: [...usersByID.keys()],
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

    // Both boxes reload on the one timer, but each renders from its own
    // result, so a failing endpoint cannot blank the other.
    const [statsResult, jobsResult] = await Promise.allSettled([
        fetchDashboardStats(controller.signal),
        fetchImportJobs(controller.signal),
    ]);

    try {
        if (controller.signal.aborted) {
            return;
        }

        if (statsResult.status === "fulfilled") {
            renderDashboard(statsResult.value);
            dashboardMetrics.setAttribute("aria-busy", "false");
            setDashboardStatus("");
            dashboardUpdated.textContent = `Updated at ${formatClock(
                new Date(),
            )}`;
        } else if (!isAbort(statsResult.reason) && !controller.signal.aborted) {
            dashboardMetrics.setAttribute("aria-busy", "false");
            setDashboardStatus(
                "Could not refresh the dashboard; retrying…",
                "error",
            );
        }

        if (jobsResult.status === "fulfilled") {
            showImportJobs(jobsResult.value?.importJobs);
        } else if (!isAbort(jobsResult.reason) && !controller.signal.aborted) {
            // Keep the rows already on screen and say the reload failed.
            setImportJobsStatus(
                "Could not load import jobs; retrying…",
                "error",
            );
        }
    } finally {
        if (dashboardController === controller) {
            dashboardController = null;
        }
    }

    scheduleDashboardRefresh();
}

async function fetchDashboardStats(signal) {
    const response = await apiFetch("/api/dashboard", {
        headers: { Accept: "application/json" },
        signal,
    });
    if (!response.ok) {
        throw await readError(response);
    }
    return response.json();
}

async function fetchImportJobs(signal) {
    const response = await apiFetch("/api/import-jobs", {
        headers: { Accept: "application/json" },
        signal,
    });
    if (!response.ok) {
        throw await readError(response);
    }
    return response.json();
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

// Stores a freshly loaded job list and shows either the empty state or the
// current page of rows.
function showImportJobs(jobs) {
    importJobs = sortImportJobs(Array.isArray(jobs) ? jobs : []);
    if (importJobs.length === 0) {
        clearImportJobs();
        setImportJobsStatus("No import jobs yet.");
        return;
    }
    setImportJobsStatus("");
    renderImportJobs();
}

function renderImportJobs() {
    if (importJobs.length === 0) {
        clearImportJobs();
        return;
    }

    importJobsPageSize.value = String(importJobsRowsPerPage);
    const totalPages = totalPageCount(importJobs.length, importJobsRowsPerPage);
    importJobsPage = clampPage(importJobsPage, totalPages);
    const start = (importJobsPage - 1) * importJobsRowsPerPage;
    const pageJobs = importJobs.slice(start, start + importJobsRowsPerPage);

    importJobsResults.replaceChildren(
        ...pageJobs.flatMap((job, index) => buildImportJobRows(job, index)),
    );
    importJobsToolbar.hidden = false;
    importJobsResultsPanel.hidden = false;
    importJobsCount.hidden = false;
    importJobsCount.textContent = `Page ${importJobsPage} of ${totalPages} · ${formatCount(
        importJobs.length,
    )} ${importJobs.length === 1 ? "job" : "jobs"}`;
    renderPagination(
        importJobsPagination,
        importJobsPage,
        totalPages,
        goToImportJobsPage,
    );
}

// The server already returns rows newest first; this keeps that order even if
// a response arrives out of sequence, breaking ties on the job ID.
function sortImportJobs(jobs) {
    return [...jobs].sort((left, right) => {
        const byCreated =
            new Date(right.createdAt ?? 0) - new Date(left.createdAt ?? 0);
        if (byCreated !== 0) {
            return byCreated;
        }
        return countValue(right.id) - countValue(left.id);
    });
}

function goToImportJobsPage(page) {
    const totalPages = totalPageCount(importJobs.length, importJobsRowsPerPage);
    if (
        !Number.isInteger(page) ||
        page < 1 ||
        page > totalPages ||
        page === importJobsPage
    ) {
        return;
    }
    importJobsPage = page;
    renderImportJobs();
}

function buildImportJobRows(job, index) {
    const jobID = String(job.id ?? "unknown");
    const row = document.createElement("tr");
    row.className = "app-row";

    const nameCell = newElement("td", "app-cell");
    const body = newElement("div", "app-cell__body");
    const info = newElement("div", "app-cell__info");

    const nameLink = newElement("a", "app-cell__name", job.steamUserName ?? "");
    nameLink.href = `/user?name=${encodeURIComponent(job.steamUserName ?? "")}`;
    nameLink.dataset.route = "/user";

    info.append(
        nameLink,
        newElement("p", "app-cell__meta", `Steam ID: ${job.steamId ?? ""}`),
    );
    body.append(info);
    nameCell.append(body);

    const countCell = newElement("td", "import-job-count");
    countCell.textContent = `${formatCount(importedAppCount(job))}/${formatCount(
        totalAppCount(job),
    )}`;

    const status = importJobStatus(job.status);
    const statusCell = newElement("td");
    statusCell.append(badge(status.label, status.tone));

    const detailsID = `import-job-details-${jobID}-${index}`;
    const detailsRow = document.createElement("tr");
    detailsRow.className = "app-details-row";
    detailsRow.hidden = true;

    const detailsCell = newElement("td", "app-details-cell");
    detailsCell.colSpan = 4;

    const details = newElement("div", "app-details");
    details.id = detailsID;
    details.append(buildImportJobDetails(job));
    detailsCell.append(details);
    detailsRow.append(detailsCell);

    const expandCell = buildExpandToggle(
        job.steamUserName ?? "import job",
        detailsID,
        detailsRow,
        {
            expanded: expandedImportJobs.has(jobID),
            onToggle: (expanded) => {
                if (expanded) {
                    expandedImportJobs.add(jobID);
                } else {
                    expandedImportJobs.delete(jobID);
                }
            },
        },
    );

    row.append(nameCell, countCell, statusCell, expandCell);
    return [row, detailsRow];
}

function buildImportJobDetails(job) {
    const details = newElement("dl", "details app-details__list");
    details.append(
        detailRow("Job ID", String(job.id ?? "")),
        detailRow("Steam user name", job.steamUserName ?? ""),
        detailRow("Steam ID", String(job.steamId ?? "")),
        detailRow("Status", importJobStatus(job.status).label),
        detailRow("Created", formatDateTime(job.createdAt) || "—"),
        detailRow("Started", formatDateTime(job.startedAt) || "—"),
        detailRow("Finished", formatDateTime(job.finishedAt) || "—"),
        detailRow("Owned apps total", formatCount(job.totalApps)),
        detailRow("Owned apps processed", formatCount(job.processedApps)),
        detailRow("Owned apps skipped", formatCount(job.skippedApps)),
        detailRow("Owned apps failed", formatCount(job.failedApps)),
        detailRow("Wishlist apps total", formatCount(job.wishlistTotalApps)),
        detailRow(
            "Wishlist apps processed",
            formatCount(job.wishlistProcessedApps),
        ),
        detailRow(
            "Wishlist apps skipped",
            formatCount(job.wishlistSkippedApps),
        ),
        detailRow("Wishlist apps failed", formatCount(job.wishlistFailedApps)),
    );
    if (job.error) {
        details.append(detailRow("Error", job.error));
    }
    return details;
}

// Job states keep their own wording: the list reports what the worker did,
// unlike importStatus, which tells a user whether their data is usable.
function importJobStatus(status) {
    switch (status) {
        case "queued":
            return { label: "Queued", tone: "queued" };
        case "running":
            return { label: "Running", tone: "running" };
        case "succeeded":
            return { label: "Succeeded", tone: "succeeded" };
        case "failed":
            return { label: "Failed", tone: "failed" };
        default:
            return { label: "Unknown", tone: "unknown" };
    }
}

// A queued job reports 0/0 until the worker records the totals.
function importedAppCount(job) {
    return (
        countValue(job.processedApps) + countValue(job.wishlistProcessedApps)
    );
}

function totalAppCount(job) {
    return countValue(job.totalApps) + countValue(job.wishlistTotalApps);
}

function clearImportJobs() {
    importJobs = [];
    importJobsResults.replaceChildren();
    importJobsResultsPanel.hidden = true;
    importJobsToolbar.hidden = true;
    importJobsCount.textContent = "";
    importJobsCount.hidden = true;
    importJobsPagination.replaceChildren();
}

function setImportJobsStatus(message, state) {
    importJobsStatus.textContent = message;
    if (state) {
        importJobsStatus.dataset.state = state;
    } else {
        delete importJobsStatus.dataset.state;
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
    user: { enter: enterUser, leave: leaveUser },
    group: { enter: enterGroup, leave: leaveGroup },
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
    const legacyRedirect = LEGACY_ROUTE_REDIRECTS.get(path);
    if (legacyRedirect !== undefined) {
        history.replaceState(
            {},
            "",
            legacyRedirect + location.search + location.hash,
        );
        renderRoute();
        return;
    }

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

    document.title = `${ROUTE_TITLES[route]} · Game Picker Utility`;

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
