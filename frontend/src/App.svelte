<script>
  import { onMount } from "svelte";
  import {
    ScanMultipleFolders,
    OrganizeFiles,
    SelectFolder,
    GetSystemPaths,
    GetOSInfo,
    GetHistory,
    UndoByID,
    LoadSchema,
    SaveSchema,
    LoadConfig,
    SaveConfig,
    ExportRules,
    ImportRulesDialog,
  } from "../wailsjs/go/main/App";

  // --- LOGIC (TIDAK DISENTUH) ---
  let osInfo = $state("");
  let folders = $state([]);
  let files = $state([]);
  let history = $state([]);
  let schema = $state({ rules: [] });
  let viewMode = $state("list");
  let conflictMode = $state("rename");
  let isScanning = $state(false);

  let showSchemaEditor = $state(false);
  let showHelper = $state(false);
  let showConflictDropdown = $state(false);
  let alert = $state({ show: false, title: "", msg: "", type: "info" });
  let confirmModal = $state({
    show: false,
    title: "",
    msg: "",
    onConfirm: null,
  });
  let importModal = $state({ show: false, rules: [] });

  const conflictOptions = [
    { id: "rename", label: "AUTO RENAME", icon: "📝" },
    { id: "overwrite", label: "OVERWRITE", icon: "⚠️" },
    { id: "skip", label: "SKIP DUPLICATE", icon: "⏭️" },
  ];

  let realConflicts = $derived.by(() => {
    const registry = new Map();
    const conflictIDs = new Set();
    schema.rules.forEach((rule) => {
      rule.extensions.forEach((ext) => {
        const uniqueKey = `${rule.pattern.toLowerCase().trim()}|${ext.toLowerCase().trim()}`;
        if (registry.has(uniqueKey)) {
          const firstMatch = registry.get(uniqueKey);
          if (firstMatch.targetDir !== rule.targetDir) {
            conflictIDs.add(rule.id);
            conflictIDs.add(firstMatch.id);
          }
        } else {
          registry.set(uniqueKey, { id: rule.id, targetDir: rule.targetDir });
        }
      });
    });
    return Array.from(conflictIDs);
  });

  onMount(async () => {
    osInfo = await GetOSInfo();
    const cfg = await LoadConfig();
    viewMode = cfg.viewMode || "list";
    conflictMode = cfg.conflictMode || "rename";
    const sysPaths = await GetSystemPaths();
    schema = await LoadSchema();
    folders = Object.entries(sysPaths).map(([name, path]) => ({
      name,
      path,
      active: cfg.watchPaths?.includes(path) || false,
    }));
    await refresh();
  });

  async function refresh() {
    isScanning = true;
    const activePaths = folders.filter((f) => f.active).map((f) => f.path);
    files = (await ScanMultipleFolders(activePaths)) || [];
    history = (await GetHistory()) || [];
    isScanning = false;
    SaveConfig({ viewMode, conflictMode, watchPaths: activePaths });
  }

  async function handleOrganize() {
    const activePaths = folders.filter((f) => f.active).map((f) => f.path);
    if (activePaths.length === 0) {
      showAlert("No Path", "Select at least one sector.", "warning");
      return;
    }
    try {
      const count = await OrganizeFiles(activePaths, schema, conflictMode);
      showAlert("Success", `${count} files sorted.`, "success");
      await refresh();
    } catch (e) {
      showAlert("Error", e.toString(), "error");
    }
  }

  function showAlert(title, msg, type) {
    alert = { show: true, title, msg, type };
  }
  function showConfirm(title, msg, onConfirm) {
    confirmModal = { show: true, title, msg, onConfirm };
  }

  async function addFolder() {
    const p = await SelectFolder();
    if (p && !folders.find((f) => f.path === p)) {
      const folderName = p.split(/[\\/]/).filter(Boolean).pop() || "Folder";
      folders = [...folders, { name: folderName, path: p, active: true }];
    }
  }

  function removeFolder(path) {
    showConfirm(
      "Remove Sector?",
      `Stop watching ${path.split(/[\\/]/).pop()}?`,
      () => {
        folders = folders.filter((f) => f.path !== path);
      },
    );
  }

  async function resetFolders() {
    showConfirm("Reset Sectors?", "Revert to default folders?", async () => {
      const sysPaths = await GetSystemPaths();
      folders = Object.entries(sysPaths).map(([name, path]) => ({
        name,
        path,
        active: true,
      }));
    });
  }

  async function deleteRule(index) {
    showConfirm("Delete Rule?", "Permanently remove this logic?", () => {
      schema.rules = schema.rules.filter((_, i) => i !== index);
    });
  }

  async function handleExport() {
    try {
      const success = await ExportRules();
      if (success)
        showAlert("Export Success", "Rules saved to JSON.", "success");
    } catch (e) {
      showAlert("Export Error", e.toString(), "error");
    }
  }

  async function handleImport() {
    try {
      const s = await ImportRulesDialog();
      if (s.rules?.length) {
        importModal = {
          show: true,
          rules: s.rules.map((r) => ({ ...r, selected: true })),
        };
      } else {
        showAlert("Import Failed", "No rules found.", "error");
      }
    } catch (e) {
      showAlert("Import Error", "Failed to read file.", "error");
    }
  }

  function applyImport() {
    const selected = importModal.rules.filter((r) => r.selected);
    const existingNames = schema.rules.map((r) => r.name);
    schema.rules = [
      ...schema.rules,
      ...selected.filter((r) => !existingNames.includes(r.name)),
    ];
    importModal.show = false;
    showAlert("Import Success", `${selected.length} rules merged.`, "success");
    refresh();
  }

  async function saveSchema() {
    await SaveSchema(schema);
    showSchemaEditor = false;
    showAlert("Saved", "Rules deployed.", "success");
  }

  $effect(() => {
    if (folders.length || viewMode) refresh();
  });
</script>

<div class="fixed inset-0 bg-[#0f1117] -z-50"></div>

<main
  class="h-screen w-screen text-slate-300 font-sans p-6 overflow-hidden flex justify-center selection:bg-indigo-500/30"
>
  <div class="w-full max-w-7xl grid grid-cols-12 gap-6 h-full">
    <aside class="col-span-3 flex flex-col gap-4 h-full overflow-hidden">
      <div
        class="bg-[#1a1d27] border border-white/5 p-6 rounded-2xl flex flex-col shrink-0"
      >
        <div class="flex justify-between items-center mb-4">
          <h2
            class="text-[10px] font-bold tracking-widest text-slate-500 uppercase"
          >
            Watch Sector
          </h2>
          <button
            onclick={resetFolders}
            class="text-[9px] font-bold text-slate-600 hover:text-red-400 transition-colors uppercase"
            >Reset</button
          >
        </div>
        <div class="space-y-1 overflow-y-auto custom-scroll max-h-56 pr-1">
          {#each folders as f}
            <div
              class="flex items-center justify-between group px-2 py-2 rounded-lg hover:bg-white/5 transition-colors"
            >
              <label
                class="flex items-center gap-3 cursor-pointer flex-1 min-w-0"
              >
                <input
                  type="checkbox"
                  bind:checked={f.active}
                  class="w-3.5 h-3.5 rounded bg-black border-slate-700 text-indigo-500 focus:ring-0"
                />
                <span
                  class="text-xs truncate {f.active
                    ? 'text-slate-200'
                    : 'text-slate-600'}">{f.name}</span
                >
              </label>
              <button
                onclick={() => removeFolder(f.path)}
                class="text-xs opacity-0 group-hover:opacity-100 text-slate-600 hover:text-red-500 px-1"
                >✕</button
              >
            </div>
          {/each}
        </div>
        <button
          onclick={addFolder}
          class="w-full mt-4 py-2 bg-slate-800 border border-white/5 rounded-lg text-[10px] font-bold text-slate-400 hover:text-white transition-colors"
        >
          + ATTACH SECTOR
        </button>
      </div>

      <div
        class="flex-1 bg-[#1a1d27] border border-white/5 p-6 rounded-2xl overflow-hidden flex flex-col"
      >
        <h2
          class="text-[10px] font-bold tracking-widest text-slate-500 mb-4 uppercase text-center italic"
        >
          History
        </h2>
        <div class="space-y-2 overflow-y-auto custom-scroll pr-2">
          {#each history as tx}
            <div class="p-3 bg-white/2 border border-white/5 rounded-lg group">
              <div class="flex justify-between items-center">
                <span class="text-[9px] font-mono text-indigo-400">{tx.id}</span
                >
                <button
                  onclick={() => UndoByID(tx.id).then(refresh)}
                  class="text-[8px] font-bold text-red-500/50 hover:text-red-500 uppercase opacity-0 group-hover:opacity-100 transition-opacity"
                  >Undo</button
                >
              </div>
              <div
                class="text-[10px] text-slate-500 mt-1 uppercase tracking-tighter"
              >
                Moved {tx.operations.length} items
              </div>
            </div>
          {/each}
        </div>
      </div>
    </aside>

    <section
      class="col-span-9 flex flex-col gap-6 h-full overflow-hidden"
      role="button"
      tabindex="0"
      onclick={() => (showConflictDropdown = false)}
      onkeydown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          showConflictDropdown = false;
        }
      }}
    >
      <header class="flex justify-between items-end px-2 shrink-0">
        <div>
          <h1 class="text-6xl font-black tracking-tighter italic text-white">
            SUPERD
          </h1>
          <p
            class="text-[10px] tracking-[0.4em] text-slate-600 font-bold uppercase mt-1 italic"
          >
            {osInfo}
          </p>
        </div>
        <div class="flex p-1 bg-[#1a1d27] rounded-xl border border-white/5">
          <button
            onclick={() => (viewMode = "grid")}
            class="px-5 py-1.5 rounded-lg text-[10px] font-bold {viewMode ===
            'grid'
              ? 'bg-indigo-600 text-white'
              : 'text-slate-500'}">GRID</button
          >
          <button
            onclick={() => (viewMode = "list")}
            class="px-5 py-1.5 rounded-lg text-[10px] font-bold {viewMode ===
            'list'
              ? 'bg-indigo-600 text-white'
              : 'text-slate-500'}">LIST</button
          >
        </div>
      </header>

      {#if files.length > 0}
        <div
          class="bg-indigo-600 rounded-3xl p-10 flex justify-between items-center relative shrink-0 z-40 overflow-visible"
        >
          <div class="z-10 text-white">
            <h3
              class="text-8xl font-black tracking-tighter leading-none italic"
            >
              {files.length}
            </h3>
            <p
              class="text-indigo-100 font-bold text-xs mt-3 uppercase tracking-widest opacity-80 italic"
            >
              Unordered Assets
            </p>
          </div>

          <div class="flex gap-4 z-10 items-center">
            <div class="relative min-w-45">
              <button
                onclick={(e) => {
                  e.stopPropagation();
                  showConflictDropdown = !showConflictDropdown;
                }}
                class="w-full bg-black/20 border border-white/10 rounded-xl px-6 py-4 text-[10px] font-bold text-white flex items-center justify-between hover:bg-black/30 transition-colors uppercase italic"
              >
                {conflictOptions.find((o) => o.id === conflictMode).label}
                <span class="opacity-50 ml-2 text-[8px]">▼</span>
              </button>
              {#if showConflictDropdown}
                <div
                  class="absolute top-full mt-2 left-0 w-full bg-[#1a1d27] border border-white/10 rounded-xl overflow-hidden z-50"
                >
                  {#each conflictOptions as opt}
                    <button
                      onclick={() => {
                        conflictMode = opt.id;
                        showConflictDropdown = false;
                      }}
                      class="w-full px-6 py-3 text-[10px] font-bold text-left hover:bg-indigo-600 hover:text-white transition-colors {conflictMode ===
                      opt.id
                        ? 'text-indigo-400 bg-white/5'
                        : 'text-slate-400'}"
                    >
                      {opt.label}
                    </button>
                  {/each}
                </div>
              {/if}
            </div>
            <button
              onclick={() => handleOrganize()}
              class="bg-white text-indigo-700 h-20 w-52 rounded-2xl font-black text-xl hover:bg-slate-100 transition-colors uppercase italic"
              >Fix Chaos</button
            >
          </div>
        </div>

        <div class="flex-1 overflow-y-auto custom-scroll min-h-0 pr-4">
          {#if viewMode === "grid"}
            <div class="grid grid-cols-5 gap-4 pb-24">
              {#each files as file}
                <div
                  class="group relative bg-[#1a1d27] border border-white/5 rounded-2xl p-5 h-fit shadow-sm"
                >
                  <div
                    class="aspect-square bg-white/3 rounded-xl mb-4 flex items-center justify-center text-2xl font-bold text-slate-700 group-hover:text-indigo-400 transition-colors"
                  >
                    {file.extension.slice(1) || "?"}
                  </div>
                  <p
                    class="text-[10px] font-bold truncate text-slate-400 text-center uppercase tracking-tighter italic"
                  >
                    {file.name}
                  </p>
                  <div
                    class="absolute inset-0 bg-[#0f1117] rounded-2xl p-4 opacity-0 group-hover:opacity-100 flex flex-col justify-center text-center z-20 border border-indigo-500/50"
                  >
                    <p
                      class="text-[9px] font-mono text-indigo-400 break-all mb-2 leading-tight uppercase font-bold"
                    >
                      {file.name}
                    </p>
                    <div
                      class="text-[8px] font-bold text-slate-600 italic border-t border-white/5 pt-2"
                    >
                      {(file.size / 1024).toFixed(0)} KB
                    </div>
                  </div>
                </div>
              {/each}
            </div>
          {:else}
            <div class="flex flex-col gap-1 pb-24">
              {#each files as file}
                <div
                  class="flex justify-between items-center p-3 px-6 bg-[#1a1d27] border border-white/5 rounded-xl hover:bg-white/4 transition-colors group"
                >
                  <div class="flex items-center gap-4 min-w-0">
                    <span
                      class="w-10 h-10 bg-black/20 rounded-lg flex items-center justify-center text-[10px] font-bold text-indigo-400 uppercase border border-white/5"
                      >{file.extension.slice(1) || "?"}</span
                    >
                    <div class="flex flex-col min-w-0">
                      <span
                        class="text-xs font-bold text-slate-300 truncate uppercase italic"
                        >{file.name}</span
                      >
                      <span
                        class="text-[8px] text-slate-600 truncate font-mono italic"
                        >{file.fullPath}</span
                      >
                    </div>
                  </div>
                  <span
                    class="text-[10px] font-mono text-slate-500 font-bold shrink-0"
                    >{(file.size / 1024).toFixed(0)} KB</span
                  >
                </div>
              {/each}
            </div>
          {/if}
        </div>
      {:else}
        <div
          class="flex-1 border border-dashed border-white/5 rounded-3xl flex flex-col items-center justify-center opacity-30"
        >
          <span
            class="text-[12rem] font-black text-slate-900 tracking-tighter select-none"
            >ZEN</span
          >
          <p
            class="text-[10px] font-bold tracking-[1em] text-slate-600 uppercase mt-2 italic"
          >
            Status: Optimized
          </p>
        </div>
      {/if}
    </section>
  </div>

  <div class="fixed bottom-10 right-10 flex flex-col gap-3 z-50">
    <button
      onclick={handleImport}
      class="w-12 h-12 bg-[#1c1f2a] border border-white/10 rounded-xl flex items-center justify-center text-slate-400 hover:text-white transition-colors"
      title="Import"
    >
      <svg
        width="20"
        height="20"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2.5"
        ><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><polyline
          points="7 10 12 15 17 10"
        /><line x1="12" x2="12" y1="15" y2="3" /></svg
      >
    </button>
    <button
      onclick={handleExport}
      class="w-12 h-12 bg-[#1c1f2a] border border-white/10 rounded-xl flex items-center justify-center text-slate-400 hover:text-white transition-colors"
      title="Export"
    >
      <svg
        width="20"
        height="20"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2.5"
        ><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><polyline
          points="17 8 12 3 7 8"
        /><line x1="12" x2="12" y1="3" y2="15" /></svg
      >
    </button>
    <button
      onclick={() => (showSchemaEditor = true)}
      class="w-14 h-14 bg-indigo-600 rounded-2xl flex items-center justify-center text-white hover:bg-indigo-500 transition-transform hover:scale-110 active:scale-95"
      title="Rules"
    >
      <svg
        width="24"
        height="24"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2.5"
        ><path
          d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.1a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"
        /><circle cx="12" cy="12" r="3" /></svg
      >
    </button>
  </div>

  {#if showSchemaEditor}
    <div
      class="fixed inset-0 z-100 flex items-center justify-center p-8 bg-black/90"
    >
      <div
        class="bg-[#12141c] border border-white/10 w-full max-w-6xl rounded-3xl p-10 flex flex-col max-h-[90vh] shadow-2xl relative overflow-hidden"
      >
        <header class="flex justify-between items-start mb-10 shrink-0">
          <div>
            <h3
              class="text-6xl font-black italic tracking-tighter text-white uppercase underline decoration-indigo-500 underline-offset-10"
            >
              Schema Architect
            </h3>
            <button
              onclick={() => (showHelper = true)}
              class="relative z-110 text-[11px] text-indigo-400 uppercase font-bold mt-10 hover:text-white transition-colors block italic underline decoration-indigo-500/30"
              >How to use patterns? ➜</button
            >
          </div>
          <div class="flex gap-4">
            <button
              onclick={() => (showSchemaEditor = false)}
              class="px-8 py-4 rounded-xl font-bold text-xs border border-white/10 text-slate-500 hover:text-white transition-colors uppercase"
              >Discard</button
            >
            <button
              onclick={saveSchema}
              class="bg-indigo-600 px-12 py-4 rounded-xl font-bold text-xs text-white hover:bg-indigo-500 transition-colors uppercase tracking-widest italic"
              >Save Logic</button
            >
          </div>
        </header>

        <div class="flex-1 overflow-y-auto custom-scroll pr-6 pb-12">
          <div class="grid grid-cols-2 gap-8">
            {#each schema.rules as rule, i}
              <div
                class="bg-[#1a1d27] border border-white/5 p-8 rounded-3xl space-y-6 relative group transition-colors"
              >
                <input
                  bind:value={rule.name}
                  class="bg-transparent text-3xl font-black text-white outline-none w-full italic border-b border-white/5 pb-3 uppercase tracking-tighter focus:border-indigo-500 transition-colors"
                  placeholder="RULE_NAME"
                />
                <div class="grid grid-cols-2 gap-6">
                  <div class="space-y-3">
                    <label
                      for="ext-pool-{rule.id}"
                      class="text-[10px] font-bold text-slate-600 block uppercase italic tracking-widest"
                      >Extension Pool</label
                    >
                    <input
                      id="ext-pool-{rule.id}"
                      value={rule.extensions.join(", ")}
                      onchange={(e) =>
                        (rule.extensions = e.target.value
                          .split(",")
                          .map((s) => s.trim()))}
                      class="w-full bg-black/30 border border-white/5 rounded-xl px-6 py-4 text-xs text-indigo-400 font-bold outline-none focus:border-indigo-500"
                    />
                    {#if realConflicts.includes(rule.id)}<p
                        class="text-[9px] text-red-500 font-bold uppercase italic tracking-widest mt-1"
                      >
                        ⚠️ Conflict: Identical Trigger
                      </p>{/if}
                  </div>
                  <div class="space-y-3">
                    <label
                      for="pattern-{rule.id}"
                      class="text-[10px] font-bold text-slate-600 block uppercase italic tracking-widest"
                      >Pattern Match</label
                    >
                    <input
                      id="pattern-{rule.id}"
                      bind:value={rule.pattern}
                      placeholder="prefix_*"
                      class="w-full bg-black/30 border border-white/5 rounded-xl px-6 py-4 text-xs text-emerald-400 font-bold outline-none focus:border-indigo-500"
                    />
                  </div>
                </div>
                <div class="space-y-3">
                  <label
                    for="target-dir-{rule.id}"
                    class="text-[10px] font-bold block uppercase italic tracking-widest text-indigo-300"
                    >Target Folder</label
                  >
                  <div class="flex gap-3">
                    <input
                      id="target-dir-{rule.id}"
                      bind:value={rule.targetDir}
                      class="flex-1 bg-black/30 border border-white/5 rounded-xl px-6 py-4 text-xs text-slate-400 outline-none uppercase font-bold"
                    />
                    <button
                      onclick={async () => {
                        const p = await SelectFolder();
                        if (p) rule.targetDir = p;
                      }}
                      class="px-8 bg-slate-800 rounded-xl hover:bg-indigo-600 transition-all text-white font-black text-xl"
                      >📂</button
                    >
                  </div>
                </div>
                <button
                  onclick={() => deleteRule(i)}
                  class="absolute top-8 right-8 text-slate-700 hover:text-red-500 transition-colors text-xl font-black"
                  >✕</button
                >
              </div>
            {/each}
            <button
              onclick={() =>
                (schema.rules = [
                  ...schema.rules,
                  {
                    id: Date.now().toString(),
                    name: "NEW RULE",
                    extensions: [".ext"],
                    pattern: "",
                    targetDir: "",
                  },
                ])}
              class="border-2 border-dashed border-white/10 rounded-3xl min-h-62.5 flex flex-col items-center justify-center text-slate-700 hover:text-indigo-500 hover:bg-white/5 transition-all"
            >
              <span class="text-6xl mb-4 font-black">+</span>
              <span class="text-[10px] uppercase font-bold italic"
                >Add New Rule</span
              >
            </button>
          </div>
        </div>
      </div>
    </div>
  {/if}

  {#if confirmModal.show}
    <div
      class="fixed inset-0 z-300 flex items-center justify-center p-8 bg-black/95"
    >
      <div
        class="bg-[#1a1d27] border border-white/10 w-full max-w-md rounded-3xl p-16 text-center animate-in zoom-in"
      >
        <div class="text-8xl mb-8 text-red-500/50 font-black italic">?</div>
        <h4
          class="text-3xl font-black italic mb-4 text-white uppercase tracking-tighter"
        >
          {confirmModal.title}
        </h4>
        <p
          class="text-xs text-slate-500 mb-10 font-bold leading-relaxed italic"
        >
          {confirmModal.msg}
        </p>
        <div class="flex flex-col gap-3">
          <button
            onclick={() => {
              confirmModal.onConfirm();
              confirmModal.show = false;
            }}
            class="w-full py-6 bg-white text-indigo-900 rounded-2xl font-black text-xs uppercase hover:bg-slate-200 transition-colors tracking-widest italic"
            >Proceed</button
          >
          <button
            onclick={() => (confirmModal.show = false)}
            class="w-full py-5 bg-slate-800 rounded-2xl font-bold text-[10px] uppercase text-slate-400 hover:text-white transition-colors italic"
            >Cancel</button
          >
        </div>
      </div>
    </div>
  {/if}

  {#if alert.show}
    <div
      class="fixed inset-0 z-400 flex items-center justify-center p-8 bg-black/95"
    >
      <div
        class="bg-[#0e1018] border border-white/10 w-md rounded-3xl p-16 text-center animate-in zoom-in"
      >
        <div class="text-9xl mb-8">
          {alert.type === "success" ? "💠" : "⚠️"}
        </div>
        <h4
          class="text-4xl font-black italic mb-4 text-white uppercase tracking-tighter"
        >
          {alert.title}
        </h4>
        <p
          class="text-sm text-slate-400 mb-14 font-bold italic tracking-wide uppercase"
        >
          {alert.msg}
        </p>
        <button
          onclick={() => (alert.show = false)}
          class="w-full py-6 bg-indigo-600 text-white rounded-2xl font-black text-xs uppercase tracking-widest hover:bg-indigo-500 transition-colors"
          >Done</button
        >
      </div>
    </div>
  {/if}

  {#if showHelper}
    <div
      class="fixed inset-0 z-200 flex items-center justify-center p-8 bg-black/95 shadow-2xl"
    >
      <div
        class="bg-[#12151f] border border-indigo-500/30 w-full max-w-2xl rounded-3xl p-20 text-left animate-in fade-in zoom-in overflow-y-auto max-h-[90vh]"
      >
        <h3
          class="text-5xl font-black mb-12 text-white uppercase tracking-tighter border-b border-white/5 pb-8 italic"
        >
          Pattern Guide
        </h3>
        <div
          class="space-y-10 text-base text-slate-400 leading-relaxed font-mono"
        >
          <div class="bg-white/5 p-8 rounded-3xl border-l-8 border-cyan-500">
            <p
              class="font-black text-slate-100 mb-4 text-xl underline decoration-cyan-500 underline-offset-4 italic"
            >
              Wildcards (*)
            </p>
            <p>
              Matches any sequence. <code
                class="text-cyan-400 bg-cyan-950/40 px-3 py-1 rounded"
                >NIM_*</code
              > matches names starting with NIM_.
            </p>
          </div>
          <div class="bg-white/5 p-8 rounded-3xl border-l-8 border-indigo-500">
            <p
              class="font-black text-slate-100 mb-4 text-xl underline decoration-indigo-500 underline-offset-4 italic"
            >
              Single Char (?)
            </p>
            <p>
              Matches one character. <code
                class="text-indigo-400 bg-indigo-950/40 px-3 py-1 rounded"
                >Doc?</code
              > matches Doc1 but not Doc10.
            </p>
          </div>
          <div class="bg-white/5 p-8 rounded-3xl border-l-8 border-amber-500">
            <p
              class="font-black text-slate-100 mb-4 text-xl underline decoration-amber-500 underline-offset-4 italic"
            >
              Priority Logic
            </p>
            <p>
              1. Length (Specific pattern wins)<br />2. Extension Pool<br />3.
              !Uncategorized
            </p>
          </div>
        </div>
        <button
          onclick={() => (showHelper = false)}
          class="mt-14 w-full py-6 bg-white text-indigo-900 rounded-2xl font-black text-xs uppercase tracking-widest italic shadow-xl"
          >Acknowledge</button
        >
      </div>
    </div>
  {/if}
</main>

<style>
  .custom-scroll::-webkit-scrollbar {
    width: 4px;
  }
  .custom-scroll::-webkit-scrollbar-thumb {
    background: #262a33;
    border-radius: 10px;
  }
  .custom-scroll::-webkit-scrollbar-thumb:hover {
    background: #333945;
  }
  .animate-in {
    animation: fadeIn 0.15s ease-out forwards;
  }
  @keyframes fadeIn {
    from {
      opacity: 0;
      transform: scale(0.98);
    }
    to {
      opacity: 1;
      transform: scale(1);
    }
  }
  :global(body) {
    background: #0f1117;
    overflow: hidden;
  }
</style>
