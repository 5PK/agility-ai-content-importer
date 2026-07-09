(function () {
	const operations = new Map();

	function operationID() {
		if (window.crypto && window.crypto.randomUUID) {
			return window.crypto.randomUUID();
		}
		return String(Date.now()) + "-" + Math.random().toString(16).slice(2);
	}

	function appID() {
		return new URLSearchParams(window.location.search).get("appID");
	}

	function invoke(operationType, arg) {
		const id = appID();
		if (!id || window.parent === window) {
			return Promise.resolve(null);
		}

		const opID = operationID();
		const payload = {
			appID: id,
			operationID: opID,
			operationType: operationType,
			arg: arg
		};

		const promise = new Promise(function (resolve) {
			operations.set(opID, resolve);
		});

		window.parent.postMessage(payload, "*");
		return promise;
	}

	function setHeight() {
		const height = Math.ceil(document.documentElement.scrollHeight);
		invoke("setHeight", { height: height });
	}

	function initializeAgility() {
		const status = document.getElementById("sdk-status");
		if (!appID()) {
			if (status) {
				status.textContent = "Preview";
			}
			setHeight();
			return;
		}

		invoke("initialize").then(function (context) {
			window.agilityContext = context || {};

			const hiddenContext = document.getElementById("content-item-json");
			if (hiddenContext) {
				hiddenContext.value = JSON.stringify(window.agilityContext.contentItem || {});
			}

			const label = document.getElementById("context-label");
			if (label && window.agilityContext.contentItem) {
				const item = window.agilityContext.contentItem;
				label.textContent = item.properties && item.properties.referenceName
					? "Import documents into " + item.properties.referenceName + "."
					: "Import documents into this content item.";
			}

			if (status) {
				status.textContent = "Ready";
				status.classList.add("is-ready");
			}
			setHeight();
		});
	}

	function setupUpload() {
		const input = document.getElementById("documents");
		const dropzone = document.getElementById("dropzone");
		const selectedFiles = document.getElementById("selected-files");
		const form = document.getElementById("upload-form");

		if (!input || !dropzone || !selectedFiles || !form) {
			return;
		}

		function updateSelectedFiles() {
			const files = Array.from(input.files || []);
			selectedFiles.textContent = files.length
				? files.map(function (file) { return file.name; }).join(", ")
				: "No files selected";
			setHeight();
		}

		input.addEventListener("change", updateSelectedFiles);

		["dragenter", "dragover"].forEach(function (eventName) {
			dropzone.addEventListener(eventName, function (event) {
				event.preventDefault();
				dropzone.classList.add("is-dragging");
			});
		});

		["dragleave", "drop"].forEach(function (eventName) {
			dropzone.addEventListener(eventName, function (event) {
				event.preventDefault();
				dropzone.classList.remove("is-dragging");
			});
		});

		dropzone.addEventListener("drop", function (event) {
			if (!event.dataTransfer || !event.dataTransfer.files.length) {
				return;
			}
			input.files = event.dataTransfer.files;
			updateSelectedFiles();
			if (window.htmx) {
				window.htmx.trigger(form, "submit");
			} else {
				form.requestSubmit();
			}
		});
	}

	window.addEventListener("message", function (event) {
		const data = event.data || {};
		if (!data.operationID || !operations.has(data.operationID)) {
			return;
		}

		const resolve = operations.get(data.operationID);
		operations.delete(data.operationID);
		resolve(data.arg || data.error || null);
	});

	document.addEventListener("DOMContentLoaded", function () {
		setupUpload();
		initializeAgility();
	});

	document.body.addEventListener("htmx:afterSettle", setHeight);
})();
