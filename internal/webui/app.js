"use strict";

const state = {
  batches: [],
  selectedID: null,
  projection: null,
  tab: "items",
  pollTimer: null,
  busy: false,
};

const labels = {
  status: {
    draft: "草稿",
    under_review: "待审查",
    correction_required: "需整改",
    ready_for_handover: "待交接",
    archived: "已归档",
  },
  review: { pending: "待审查", approved: "已通过", rejected: "已退回" },
  hazards: {
    flammable: "易燃", corrosive: "腐蚀", toxic: "有毒", oxidizing: "氧化性",
    reactive: "反应性", infectious: "感染性", environmental: "环境危害",
  },
  categories: { solid: "固体", liquid: "液体", container: "独立容器" },
};

const elements = {};

document.addEventListener("DOMContentLoaded", () => {
  [
    "connectionState", "roleSelect", "actorInput", "refreshButton", "notice", "batchCount",
    "newBatchButton", "emptyNewButton", "batchSearch", "dueFilter", "batchList", "emptyState", "batchWorkspace",
    "batchLab", "batchStatus", "batchMeta", "batchID", "batchVersion", "itemMetric", "packageMetric",
    "reviewMetric", "rejectMetric", "batchActions", "itemRows", "timelineList", "receiptContent",
    "itemsPanel", "timelinePanel", "receiptPanel", "compatibilityPanel", "itemFilter", "selectionCount", "selectAllItems", "bulkPackageButton", "bulkApproveButton", "bulkRejectButton", "batchDialog", "batchForm", "rescheduleDialog", "rescheduleForm", "bulkPackageDialog", "bulkPackageForm", "bulkReviewDialog", "bulkReviewForm", "itemDialog", "itemForm",
    "packageDialog", "packageForm", "reviewDialog", "reviewForm", "correctionDialog", "correctionForm",
    "confirmDialog", "confirmForm",
  ].forEach((id) => { elements[id] = document.getElementById(id); });

  bindEvents();
  restoreSession();
  loadBatches();
  state.pollTimer = window.setInterval(pollSelected, 8000);
});

function bindEvents() {
  elements.refreshButton.addEventListener("click", loadBatches);
  elements.newBatchButton.addEventListener("click", () => openDialog(elements.batchDialog));
  elements.emptyNewButton.addEventListener("click", () => openDialog(elements.batchDialog));
  elements.batchSearch.addEventListener("input", renderBatchList);
  elements.dueFilter.addEventListener("change", loadBatches);
  elements.itemFilter.addEventListener("change", renderItems);
  elements.selectAllItems.addEventListener("change", toggleAllItems);
  elements.bulkPackageButton.addEventListener("click", () => openBulkPackageDialog());
  elements.bulkApproveButton.addEventListener("click", () => openBulkReviewDialog("approve"));
  elements.bulkRejectButton.addEventListener("click", () => openBulkReviewDialog("reject"));
  elements.batchList.addEventListener("click", (event) => {
    const button = event.target.closest("[data-batch-id]");
    if (button) selectBatch(button.dataset.batchId);
  });
  elements.roleSelect.addEventListener("change", () => {
    elements.actorInput.value = elements.roleSelect.value === "reviewer" ? "李复核" : "张安全";
    saveSession();
    if (state.selectedID) selectBatch(state.selectedID, false);
  });
  elements.actorInput.addEventListener("change", saveSession);
  document.querySelectorAll(".tab").forEach((tab) => tab.addEventListener("click", () => switchTab(tab.dataset.tab)));
  document.querySelectorAll(".dialog-close").forEach((button) => button.addEventListener("click", () => button.closest("dialog").close()));
  elements.batchActions.addEventListener("click", handleBatchAction);
  elements.itemRows.addEventListener("click", handleItemAction);
  elements.itemRows.addEventListener("change", (event) => { if (event.target.matches(".item-select")) updateSelectionCount(); });
  elements.batchForm.addEventListener("submit", submitBatch);
  elements.itemForm.addEventListener("submit", submitItem);
  elements.packageForm.addEventListener("submit", submitPackage);
  elements.reviewForm.addEventListener("submit", submitReview);
  elements.correctionForm.addEventListener("submit", submitCorrection);
  elements.confirmForm.addEventListener("submit", submitConfirmation);
  elements.rescheduleForm.addEventListener("submit", submitReschedule);
  elements.bulkPackageForm.addEventListener("submit", submitBulkPackage);
  elements.bulkReviewForm.addEventListener("submit", submitBulkReview);
}

function restoreSession() {
  const role = localStorage.getItem("handover.role");
  const actor = localStorage.getItem("handover.actor");
  if (role === "reviewer" || role === "safety_officer") elements.roleSelect.value = role;
  if (actor) elements.actorInput.value = actor;
}

function saveSession() {
  localStorage.setItem("handover.role", elements.roleSelect.value);
  localStorage.setItem("handover.actor", elements.actorInput.value.trim());
}

function requestID(prefix) {
  const random = globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  return `${prefix}-${random}`;
}

async function api(path, options = {}) {
  const headers = new Headers(options.headers || {});
  headers.set("Accept", "application/json");
  headers.set("X-Role", elements.roleSelect.value);
  if (options.body) headers.set("Content-Type", "application/json");
  const response = await fetch(path, { ...options, headers });
  let payload;
  try {
    payload = await response.json();
  } catch (_) {
    throw new Error(`服务返回了无法解析的响应（HTTP ${response.status}）`);
  }
  if (!response.ok) {
    const error = new Error(payload.error?.message || `请求失败（HTTP ${response.status}）`);
    error.code = payload.error?.code;
    error.field = payload.error?.field;
    error.actualVersion = payload.error?.actualVersion;
    throw error;
  }
  return payload;
}

async function loadBatches() {
  if (state.busy) return;
  setConnection("正在刷新批次", false);
  try {
    const due = elements.dueFilter.value ? `&dueStatus=${encodeURIComponent(elements.dueFilter.value)}` : "";
    const payload = await api(`/api/batches?limit=100${due}`);
    state.batches = payload.data || [];
    renderBatchList();
    setConnection("本地服务已连接", true);
    if (state.selectedID) {
      const stillExists = state.batches.some((batch) => batch.id === state.selectedID);
      if (stillExists) await selectBatch(state.selectedID, false);
      else clearSelection();
    }
  } catch (error) {
    setConnection("本地服务连接失败", false);
    showNotice(error.message, true);
  }
}

async function pollSelected() {
  if (!state.selectedID || state.busy || document.hidden) return;
  try {
    await selectBatch(state.selectedID, false, true);
  } catch (_) {
    // The next explicit refresh presents the error without interrupting active form work.
  }
}

function setConnection(text, connected) {
  elements.connectionState.textContent = text;
  elements.connectionState.dataset.connected = String(connected);
}

function showNotice(message, isError = false) {
  elements.notice.textContent = message;
  elements.notice.classList.toggle("error", isError);
  elements.notice.hidden = false;
  window.clearTimeout(showNotice.timer);
  showNotice.timer = window.setTimeout(() => { elements.notice.hidden = true; }, 4500);
}

function renderBatchList() {
  const query = elements.batchSearch.value.trim().toLowerCase();
  const filtered = state.batches.filter((batch) => `${batch.sourceLab} ${batch.ownerName} ${batch.id}`.toLowerCase().includes(query));
  elements.batchCount.textContent = `${state.batches.length} 个批次`;
  elements.batchList.replaceChildren(...filtered.map((batch) => {
    const button = document.createElement("button");
    button.type = "button";
    button.className = `batch-entry${batch.id === state.selectedID ? " active" : ""}`;
    button.dataset.batchId = batch.id;
    button.innerHTML = `
      <div class="row"><strong>${escapeHTML(batch.sourceLab)}</strong><span class="status-chip status-${batch.status}">${labels.status[batch.status] || batch.status}</span></div>
      <div class="due-chip due-${batch.dueStatus || "normal"}">${dueLabel(batch.dueStatus)}</div>
      <div class="row"><small>${escapeHTML(batch.ownerName)} · ${batch.items.length} 项</small><small>v${batch.version}</small></div>`;
    return button;
  }));
}

async function selectBatch(id, announce = true, quiet = false) {
  state.selectedID = id;
  renderBatchList();
  try {
    const payload = await api(`/api/batches/${encodeURIComponent(id)}`);
    state.projection = payload.data;
    renderWorkspace();
    if (state.tab === "timeline") await renderTimeline();
    if (state.tab === "receipt") await renderReceipt();
    if (announce) showNotice("已载入批次最新版本");
  } catch (error) {
    if (!quiet) showNotice(error.message, true);
    throw error;
  }
}

function clearSelection() {
  state.selectedID = null;
  state.projection = null;
  elements.emptyState.hidden = false;
  elements.batchWorkspace.hidden = true;
  renderBatchList();
}

function renderWorkspace() {
  const batch = state.projection.batch;
  elements.emptyState.hidden = true;
  elements.batchWorkspace.hidden = false;
  elements.batchLab.textContent = batch.sourceLab;
  elements.batchStatus.textContent = labels.status[batch.status] || batch.status;
  elements.batchStatus.className = `status-chip status-${batch.status}`;
  elements.batchMeta.textContent = `责任人：${batch.ownerName} · 计划交接：${formatDate(batch.plannedHandoverAt)}`;
  elements.batchID.textContent = batch.id;
  elements.batchVersion.textContent = batch.version;
  const packageCount = batch.items.filter((item) => item.containerType && item.sealChecked && item.labelChecked).length;
  const approvedCount = batch.items.filter((item) => item.reviewStatus === "approved").length;
  elements.itemMetric.textContent = batch.items.length;
  elements.packageMetric.textContent = `${packageCount} / ${batch.items.length}`;
  elements.reviewMetric.textContent = `${approvedCount} / ${batch.items.length}`;
  elements.rejectMetric.textContent = state.projection.rejectedCount;
  renderStages(batch.status);
  renderActions();
  renderItems();
  renderCompatibility();
  renderBatchList();
}

function renderStages(status) {
  const order = { draft: 0, under_review: 1, correction_required: 1, ready_for_handover: 2, archived: 3 };
  const current = order[status] ?? 0;
  document.querySelectorAll(".stage-track li").forEach((stage, index) => {
    stage.classList.toggle("done", index < current || status === "archived");
    stage.classList.toggle("current", index === current && status !== "archived");
  });
}

function renderActions() {
  const allowed = new Set(state.projection.allowedActions || []);
  const actions = [];
  if (allowed.has("add_item")) actions.push(actionButton("add-item", "登记条目", "primary"));
  if (allowed.has("submit_review")) actions.push(actionButton("submit-review", "提交审查", "primary"));
  if (allowed.has("complete_review")) actions.push(actionButton("complete-review", "完成本轮审查", "secondary"));
  if (allowed.has("freeze_manifest")) actions.push(actionButton("freeze", "冻结交接清单", "primary"));
  if (allowed.has("confirm_handover")) actions.push(actionButton("confirm", "确认现场交接", "danger"));
  if (allowed.has("view_receipt")) actions.push(actionButton("view-receipt", "查看归档凭据", "secondary"));
  if (elements.roleSelect.value === "safety_officer" && ["draft", "correction_required"].includes(state.projection.batch.status)) actions.push(actionButton("reschedule", "调整计划日期", "secondary"));
  elements.batchActions.replaceChildren(...actions);
  const bulkAllowed = allowed.has("set_package") || allowed.has("correct_item");
  elements.bulkPackageButton.hidden = !bulkAllowed;
  elements.bulkApproveButton.hidden = !allowed.has("review_item");
  elements.bulkRejectButton.hidden = !allowed.has("review_item");
}

function actionButton(action, text, style) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = style;
  button.dataset.action = action;
  button.textContent = text;
  return button;
}

function renderItems() {
  const batch = state.projection.batch;
  const allowed = new Set(state.projection.allowedActions || []);
  if (!batch.items.length) {
    const row = document.createElement("tr");
    row.innerHTML = `<td colspan="7" class="empty-row">当前批次还没有废弃物条目</td>`;
    elements.itemRows.replaceChildren(row);
    return;
  }
  const filter = elements.itemFilter.value;
  const rows = batch.items.filter((item) => filter === "package_missing" ? !item.containerType || !item.sealChecked || !item.labelChecked : !filter || item.reviewStatus === filter).map((item) => {
    const row = document.createElement("tr");
    const itemActions = [];
    if (allowed.has("update_item")) itemActions.push(`<button class="table-action" data-action="edit-item" data-item-id="${item.id}">编辑</button>`);
    if (allowed.has("set_package")) itemActions.push(`<button class="table-action" data-action="package" data-item-id="${item.id}">封装核验</button>`);
    if (allowed.has("review_item")) itemActions.push(`<button class="table-action" data-action="review" data-item-id="${item.id}">审查</button>`);
    if (allowed.has("correct_item") && item.reviewStatus === "rejected") itemActions.push(`<button class="table-action" data-action="correct" data-item-id="${item.id}">整改</button>`);
    row.innerHTML = `
      <td><input type="checkbox" class="item-select" data-item-id="${item.id}" ${isSelectable(item) ? "" : "disabled"}></td>
      <td><strong class="material-name">${escapeHTML(item.materialName)}</strong><div class="hazard-list">${item.hazardClasses.map((hazard) => `<span class="hazard">${labels.hazards[hazard] || escapeHTML(hazard)}</span>`).join("")}</div></td>
      <td>${formatNumber(item.quantity)} ${escapeHTML(item.unit)}<br><small>${labels.categories[item.disposalCategory] || escapeHTML(item.disposalCategory)}</small></td>
      <td>${item.containerType ? escapeHTML(item.containerType) : "<small>未填写</small>"}</td>
      <td><span class="check-state ${item.sealChecked ? "ok" : ""}">${item.sealChecked ? "✓" : "○"} 封口</span><span class="check-state ${item.labelChecked ? "ok" : ""}">${item.labelChecked ? "✓" : "○"} 标签</span></td>
      <td><span class="review-chip review-${item.reviewStatus}">${labels.review[item.reviewStatus]}</span>${item.correctionNote ? `<small class="check-state">${escapeHTML(item.correctionNote)}</small>` : ""}</td>
      <td><div class="row-actions">${itemActions.join("")}</div></td>`;
    return row;
  });
  elements.itemRows.replaceChildren(...rows);
  updateSelectionCount();
}

function dueLabel(value) { return ({ overdue: "已逾期", due_today: "今日到期", due_soon: "三日内临期", normal: "正常" }[value] || "正常"); }
function isSelectable(item) {
  const status = state.projection.batch.status;
  const allowed = new Set(state.projection.allowedActions || []);
  if (status === "draft") return allowed.has("set_package");
  if (status === "under_review") return allowed.has("review_item") && item.reviewStatus === "pending";
  return allowed.has("correct_item") && status === "correction_required" && item.reviewStatus === "rejected";
}
function selectedItems() { return [...document.querySelectorAll(".item-select:checked")].map((input) => input.dataset.itemId); }
function toggleAllItems() { document.querySelectorAll(".item-select:not(:disabled)").forEach((input) => { input.checked = elements.selectAllItems.checked; }); updateSelectionCount(); }
function updateSelectionCount() { elements.selectionCount.textContent = `已选择 ${selectedItems().length} 项`; }
function renderCompatibility() {
  const report = state.projection.compatibility;
  if (!report) { elements.compatibilityPanel.replaceChildren(); return; }
  const findings = (report.findings || []).map((finding) => `<li class="risk-${finding.riskLevel}"><strong>${escapeHTML(finding.ruleCode)}</strong> · 条目 ${finding.itemIds.map(escapeHTML).join(", ")}：${escapeHTML(finding.remediation)}</li>`).join("");
  elements.compatibilityPanel.innerHTML = `<strong>相容性预检 · 规则 ${escapeHTML(report.ruleVersion)}</strong><span>${escapeHTML(report.summary)}</span>${findings ? `<ul>${findings}</ul>` : "<span>未发现冲突</span>"}`;
}

async function handleBatchAction(event) {
  const action = event.target.closest("[data-action]")?.dataset.action;
  if (!action || state.busy) return;
  if (action === "add-item") return openItemDialog();
  if (action === "confirm") {
    elements.confirmForm.elements.name.value = elements.actorInput.value.trim();
    return openDialog(elements.confirmDialog);
  }
  if (action === "view-receipt") return switchTab("receipt");
  if (action === "reschedule") return openRescheduleDialog();
  const messages = {
    "submit-review": "确认将当前批次提交相容性审查？",
    "complete-review": "确认本轮所有条目均已给出结论？",
    freeze: "确认冻结交接清单？冻结后条目和封装信息不可再修改。",
  };
  if (!window.confirm(messages[action])) return;
  const paths = {
    "submit-review": "submit-review",
    "complete-review": "complete-review",
    freeze: "freeze",
  };
  await runMutation(`/api/batches/${state.selectedID}/${paths[action]}`, baseWrite(), `${action} 已完成`);
}

function openRescheduleDialog() {
  const form = elements.rescheduleForm;
  form.reset();
  form.elements.plannedDate.value = new Date(state.projection.batch.plannedHandoverAt).toISOString().slice(0, 10);
  openDialog(elements.rescheduleDialog);
}

function openBulkPackageDialog() {
  if (!selectedItems().length) return showNotice("请先选择可核验条目", true);
  elements.bulkPackageForm.reset(); openDialog(elements.bulkPackageDialog);
}

function openBulkReviewDialog(decision) {
  if (!selectedItems().length) return showNotice("请先选择条目", true);
  elements.bulkReviewForm.reset(); elements.bulkReviewForm.elements.decision.value = decision;
  document.getElementById("bulkReviewTitle").textContent = decision === "approve" ? "批量通过" : "批量退回";
  document.getElementById("bulkReasonField").hidden = decision !== "reject";
  document.getElementById("bulkCommentField").hidden = decision !== "reject";
  openDialog(elements.bulkReviewDialog);
}

function handleItemAction(event) {
  const button = event.target.closest("[data-action][data-item-id]");
  if (!button) return;
  const item = state.projection.batch.items.find((entry) => entry.id === button.dataset.itemId);
  if (!item) return;
  switch (button.dataset.action) {
    case "edit-item": openItemDialog(item); break;
    case "package": openPackageDialog(item); break;
    case "review": openReviewDialog(item); break;
    case "correct": openCorrectionDialog(item); break;
  }
}

function openDialog(dialog) {
  clearFormError(dialog.querySelector("form"));
  dialog.showModal();
}

function openItemDialog(item = null) {
  const form = elements.itemForm;
  form.reset();
  form.elements.itemID.value = item?.id || "";
  document.getElementById("itemDialogTitle").textContent = item ? "编辑废弃物条目" : "登记废弃物条目";
  if (item) {
    form.elements.materialName.value = item.materialName;
    form.elements.quantity.value = item.quantity;
    form.elements.unit.value = item.unit;
    form.elements.disposalCategory.value = item.disposalCategory;
    form.querySelectorAll("input[name=hazards]").forEach((input) => { input.checked = item.hazardClasses.includes(input.value); });
  }
  openDialog(elements.itemDialog);
}

function openPackageDialog(item) {
  const form = elements.packageForm;
  form.reset();
  form.elements.itemID.value = item.id;
  form.elements.containerType.value = item.containerType || "";
  form.elements.sealChecked.checked = item.sealChecked;
  form.elements.labelChecked.checked = item.labelChecked;
  document.getElementById("packageItemName").textContent = item.materialName;
  openDialog(elements.packageDialog);
}

function openReviewDialog(item) {
  const form = elements.reviewForm;
  form.reset();
  form.elements.itemID.value = item.id;
  form.elements.reviewerName.value = elements.actorInput.value.trim();
  document.getElementById("reviewItemName").textContent = `${item.materialName} · ${formatNumber(item.quantity)} ${item.unit}`;
  openDialog(elements.reviewDialog);
}

function openCorrectionDialog(item) {
  const form = elements.correctionForm;
  form.reset();
  form.elements.itemID.value = item.id;
  form.elements.materialName.value = item.materialName;
  form.elements.hazards.value = item.hazardClasses.join(", ");
  form.elements.quantity.value = item.quantity;
  form.elements.unit.value = item.unit;
  form.elements.disposalCategory.value = item.disposalCategory;
  form.elements.containerType.value = item.containerType || "";
  form.elements.sealChecked.checked = item.sealChecked;
  form.elements.labelChecked.checked = item.labelChecked;
  document.getElementById("correctionReason").textContent = item.correctionNote || "请按审查意见完成整改";
  openDialog(elements.correctionDialog);
}

async function submitBatch(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const values = new FormData(form);
  const date = new Date(`${values.get("plannedDate")}T12:00:00`);
  const body = {
    requestId: requestID("create"), sourceLab: values.get("sourceLab"),
    ownerName: values.get("ownerName"), plannedHandoverAt: date.toISOString(),
  };
  try {
    setBusy(true);
    const payload = await api("/api/batches", { method: "POST", body: JSON.stringify(body) });
    elements.batchDialog.close();
    state.selectedID = payload.data.id;
    await loadBatches();
    showNotice("交接批次已创建");
  } catch (error) { setFormError(form, error); }
  finally { setBusy(false); }
}

async function submitItem(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const values = new FormData(form);
  const itemID = values.get("itemID");
  const body = {
    ...baseWrite(), materialName: values.get("materialName"),
    hazardClasses: values.getAll("hazards"), quantity: Number(values.get("quantity")),
    unit: values.get("unit"), disposalCategory: values.get("disposalCategory"),
  };
  const path = itemID ? `/api/batches/${state.selectedID}/items/${itemID}` : `/api/batches/${state.selectedID}/items`;
  const method = itemID ? "PUT" : "POST";
  await submitDialogMutation(form, elements.itemDialog, path, body, itemID ? "条目已更新" : "条目已登记", method);
}

async function submitPackage(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const values = new FormData(form);
  const body = {
    ...baseWrite(), containerType: values.get("containerType"),
    sealChecked: form.elements.sealChecked.checked, labelChecked: form.elements.labelChecked.checked,
  };
  await submitDialogMutation(form, elements.packageDialog, `/api/batches/${state.selectedID}/items/${values.get("itemID")}/package`, body, "封装核验已保存", "PUT");
}

async function submitReview(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const values = new FormData(form);
  const body = {
    ...baseWrite(), decision: values.get("decision"), reasonCode: values.get("reasonCode"),
    comment: values.get("comment"), reviewerName: values.get("reviewerName"),
  };
  await submitDialogMutation(form, elements.reviewDialog, `/api/batches/${state.selectedID}/reviews/${values.get("itemID")}`, body, "审查结论已记录");
}

async function submitCorrection(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const values = new FormData(form);
  const body = {
    ...baseWrite(), materialName: values.get("materialName"),
    hazardClasses: String(values.get("hazards")).split(",").map((value) => value.trim()).filter(Boolean),
    quantity: Number(values.get("quantity")), unit: values.get("unit"), disposalCategory: values.get("disposalCategory"),
    containerType: values.get("containerType"), sealChecked: form.elements.sealChecked.checked,
    labelChecked: form.elements.labelChecked.checked, note: values.get("note"),
  };
  await submitDialogMutation(form, elements.correctionDialog, `/api/batches/${state.selectedID}/corrections/${values.get("itemID")}`, body, "整改信息已保存");
}

async function submitConfirmation(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const values = new FormData(form);
  await submitDialogMutation(form, elements.confirmDialog, `/api/batches/${state.selectedID}/confirmations`, { ...baseWrite(), name: values.get("name") }, "现场交接确认已提交");
}

async function submitReschedule(event) {
  event.preventDefault();
  const form = event.currentTarget; const values = new FormData(form);
  const date = new Date(`${values.get("plannedDate")}T12:00:00`);
  await submitDialogMutation(form, elements.rescheduleDialog, `/api/batches/${state.selectedID}/reschedule`, { ...baseWrite(), plannedHandoverAt: date.toISOString(), reason: values.get("reason") }, "计划交接日期已调整");
}

async function submitBulkPackage(event) {
  event.preventDefault();
  const form = event.currentTarget; const values = new FormData(form);
  const items = selectedItems().map((itemId) => ({ itemId, containerType: values.get("containerType"), sealChecked: form.elements.sealChecked.checked, labelChecked: form.elements.labelChecked.checked }));
  await submitDialogMutation(form, elements.bulkPackageDialog, `/api/batches/${state.selectedID}/packages/bulk`, { ...baseWrite(), items }, "批量封装核验已保存");
}

async function submitBulkReview(event) {
  event.preventDefault();
  const form = event.currentTarget; const values = new FormData(form); const decision = values.get("decision");
  const items = selectedItems().map((itemId) => ({ itemId, decision, reasonCode: decision === "reject" ? values.get("reasonCode") : "", comment: decision === "reject" ? values.get("comment") : "", reviewerName: elements.actorInput.value.trim() }));
  await submitDialogMutation(form, elements.bulkReviewDialog, `/api/batches/${state.selectedID}/reviews/bulk`, { ...baseWrite(), items }, "批量审查结论已记录");
}

function baseWrite() {
  return {
    requestId: requestID("write"),
    expectedVersion: state.projection.batch.version,
    actor: elements.actorInput.value.trim(),
  };
}

async function submitDialogMutation(form, dialog, path, body, success, method = "POST") {
  try {
    setBusy(true);
    await api(path, { method, body: JSON.stringify(body), headers: { "X-Request-ID": body.requestId } });
    dialog.close();
    await refreshAfterMutation(success);
  } catch (error) {
    setFormError(form, error);
    if (error.code === "version_conflict") await selectBatch(state.selectedID, false);
  } finally { setBusy(false); }
}

async function runMutation(path, body, success) {
  try {
    setBusy(true);
    await api(path, { method: "POST", body: JSON.stringify(body), headers: { "X-Request-ID": body.requestId } });
    await refreshAfterMutation(success);
  } catch (error) {
    showNotice(error.message, true);
    if (error.code === "version_conflict") await selectBatch(state.selectedID, false);
  } finally { setBusy(false); }
}

async function refreshAfterMutation(message) {
  await selectBatch(state.selectedID, false);
  const index = state.batches.findIndex((batch) => batch.id === state.selectedID);
  if (index >= 0) state.batches[index] = state.projection.batch;
  else state.batches.unshift(state.projection.batch);
  renderBatchList();
  showNotice(message);
}

function setBusy(value) {
  state.busy = value;
  document.querySelectorAll("button[type=submit], [data-action]").forEach((button) => { button.disabled = value; });
}

function setFormError(form, error) {
  const target = form.querySelector("[data-error]");
  target.textContent = error.field ? `${error.field}：${error.message}` : error.message;
  if (error.field && form.elements[error.field]) form.elements[error.field].focus();
}

function clearFormError(form) {
  const target = form.querySelector("[data-error]");
  if (target) target.textContent = "";
}

async function switchTab(tab) {
  state.tab = tab;
  document.querySelectorAll(".tab").forEach((button) => button.classList.toggle("active", button.dataset.tab === tab));
  elements.itemsPanel.hidden = tab !== "items";
  elements.timelinePanel.hidden = tab !== "timeline";
  elements.receiptPanel.hidden = tab !== "receipt";
  if (tab === "timeline") await renderTimeline();
  if (tab === "receipt") await renderReceipt();
}

async function renderTimeline() {
  if (!state.selectedID) return;
  try {
    const payload = await api(`/api/batches/${state.selectedID}/timeline`);
    const events = payload.data || [];
    elements.timelineList.replaceChildren(...events.map((event) => {
      const item = document.createElement("li");
      item.innerHTML = `<strong>${escapeHTML(event.message)}</strong><span>${escapeHTML(event.actor)} · ${formatDateTime(event.at)} · ${escapeHTML(event.type)}</span>`;
      return item;
    }));
  } catch (error) { showNotice(error.message, true); }
}

async function renderReceipt() {
  const batch = state.projection?.batch;
  if (!batch || batch.status !== "archived") {
    elements.receiptContent.innerHTML = `<div class="receipt-placeholder">批次在双方完成现场确认后签发归档凭据。</div>`;
    return;
  }
  try {
    const payload = await api(`/api/batches/${state.selectedID}/receipt`);
    const verificationPayload = await api(`/api/batches/${state.selectedID}/receipt/verification`);
    const receipt = payload.data;
    const verification = verificationPayload.data;
    elements.receiptContent.innerHTML = `
      <article class="receipt-sheet">
        <h4>实验室废弃物交接归档凭据</h4>
        <small>${escapeHTML(receipt.id)} · 签发于 ${formatDateTime(receipt.issuedAt)}</small>
        <div class="receipt-grid">
          <div><span>交接批次</span><strong>${escapeHTML(receipt.batchId)}</strong></div>
          <div><span>移交确认</span><strong>${escapeHTML(receipt.senderConfirmation.name)}<br>${formatDateTime(receipt.senderConfirmation.confirmedAt)}</strong></div>
          <div><span>接收确认</span><strong>${escapeHTML(receipt.receiverConfirmation.name)}<br>${formatDateTime(receipt.receiverConfirmation.confirmedAt)}</strong></div>
        </div>
        <div class="digest"><span>冻结清单 SHA-256</span><code>${escapeHTML(receipt.manifestDigest)}</code></div>
        <div class="digest"><span>完整时间线 SHA-256</span><code>${escapeHTML(receipt.timelineDigest)}</code></div>
        <div class="verification ${verification.passed ? "ok" : "error"}"><strong>${verification.passed ? "凭据核验通过" : "凭据核验未通过"}</strong>${verification.checks.map((check) => `<span>${check.passed ? "✓" : "×"} ${escapeHTML(check.message)}</span>`).join("")}</div>
        <a class="button secondary" href="/api/batches/${encodeURIComponent(state.selectedID)}/receipt/download">下载凭据与核验 JSON</a>
      </article>`;
  } catch (error) {
    elements.receiptContent.innerHTML = `<div class="receipt-placeholder">${escapeHTML(error.message)}</div>`;
  }
}

function formatDate(value) {
  return new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit" }).format(new Date(value));
}

function formatDateTime(value) {
  return new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false }).format(new Date(value));
}

function formatNumber(value) {
  return new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 3 }).format(value);
}

function escapeHTML(value) {
  return String(value ?? "").replace(/[&<>'"]/g, (character) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "'": "&#39;", '"': "&quot;" })[character]);
}
