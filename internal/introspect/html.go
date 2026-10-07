package introspect

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
)

// RenderHTML generates a modern interactive developer console and contract tester for browser-based API introspection.
func RenderHTML(view *Response) string {
	viewBytes, _ := json.Marshal(view)
	safeJSON := strings.ReplaceAll(string(viewBytes), "</script>", "<\\/script>")

	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>nxp API Explorer - ` + html.EscapeString(view.Path) + `</title>
  <style>
    :root {
      --bg: #0b0f19;
      --card-bg: #111827;
      --card-border: #1f2937;
      --input-bg: #1f2937;
      --input-border: #374151;
      --text: #f3f4f6;
      --text-muted: #9ca3af;
      --primary: #3b82f6;
      --primary-hover: #2563eb;
      --success: #10b981;
      --warning: #f59e0b;
      --danger: #ef4444;
      --font-mono: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
      --font-sans: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      background-color: var(--bg);
      color: var(--text);
      font-family: var(--font-sans);
      line-height: 1.5;
      padding: 24px;
    }
    .container { max-width: 1200px; margin: 0 auto; display: flex; flex-direction: column; gap: 20px; }
    
    /* Top Bar */
    .topbar {
      display: flex;
      justify-content: space-between;
      align-items: center;
      background: var(--card-bg);
      border: 1px solid var(--card-border);
      border-radius: 12px;
      padding: 16px 24px;
    }
    .brand { display: flex; align-items: center; gap: 12px; }
    .brand-logo {
      background: linear-gradient(135deg, #3b82f6, #8b5cf6);
      color: white;
      font-weight: 800;
      font-size: 14px;
      padding: 6px 12px;
      border-radius: 6px;
      letter-spacing: 0.5px;
    }
    .brand-title { font-size: 18px; font-weight: 600; }
    .format-pills { display: flex; gap: 8px; }
    .pill {
      font-size: 13px;
      padding: 6px 12px;
      border-radius: 6px;
      text-decoration: none;
      color: var(--text-muted);
      border: 1px solid var(--input-border);
      background: var(--input-bg);
      transition: all 0.15s ease;
    }
    .pill:hover { color: white; border-color: var(--primary); }
    .pill.active { background: var(--primary); color: white; border-color: var(--primary); font-weight: 600; }
    
    /* Path Header Card */
    .card {
      background: var(--card-bg);
      border: 1px solid var(--card-border);
      border-radius: 12px;
      padding: 20px 24px;
    }
    .path-row { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; margin-bottom: 12px; }
    .breadcrumbs {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      font-size: 18px;
      font-weight: 700;
      font-family: var(--font-mono);
      background: #0d131f;
      padding: 6px 14px;
      border-radius: 8px;
      border: 1px solid #1f2937;
    }
    .breadcrumb-item {
      color: #60a5fa;
      text-decoration: none;
      transition: color 0.15s ease;
      padding: 2px 6px;
      border-radius: 4px;
    }
    .breadcrumb-item:hover {
      background: #1e293b;
      color: #93c5fd;
      text-decoration: underline;
    }
    .breadcrumb-item.current {
      color: #f3f4f6;
      cursor: default;
      text-decoration: none;
    }
    .breadcrumb-sep {
      color: #6b7280;
      user-select: none;
    }
    .badge {
      font-size: 12px;
      font-weight: 700;
      padding: 3px 8px;
      border-radius: 4px;
      text-transform: uppercase;
      font-family: var(--font-mono);
    }
    .badge-get { background: #064e3b; color: #34d399; }
    .badge-post { background: #1e3a8a; color: #60a5fa; }
    .badge-put { background: #78350f; color: #fbbf24; }
    .badge-delete { background: #7f1d1d; color: #f87171; }
    .badge-patch { background: #581c87; color: #c084fc; }
    .badge-auth { background: #374151; color: #d1d5db; }
    .meta-text { font-size: 13px; color: var(--text-muted); }

    /* Children Sub-Routes */
    .children-nav { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 10px; }
    .child-link {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      background: #1f2937;
      border: 1px solid #374151;
      padding: 5px 10px;
      border-radius: 6px;
      color: #93c5fd;
      text-decoration: none;
      font-family: var(--font-mono);
      font-size: 13px;
      transition: all 0.15s ease;
    }
    .child-link:hover { border-color: #60a5fa; background: #263346; }

    /* Tabs */
    .tabs { display: flex; gap: 8px; border-bottom: 1px solid var(--card-border); margin-bottom: 20px; }
    .tab-btn {
      padding: 10px 18px;
      background: none;
      border: none;
      border-bottom: 2px solid transparent;
      color: var(--text-muted);
      font-size: 14px;
      font-weight: 600;
      cursor: pointer;
      transition: all 0.15s ease;
    }
    .tab-btn:hover { color: var(--text); }
    .tab-btn.active { color: var(--primary); border-bottom-color: var(--primary); }
    .tab-content { display: none; }
    .tab-content.active { display: block; }

    /* Layout Split */
    .split-layout { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; }
    @media (max-width: 900px) { .split-layout { grid-template-columns: 1fr; } }

    /* Form & Request UI */
    .form-group { margin-bottom: 16px; }
    .form-label {
      display: block;
      font-size: 13px;
      font-weight: 600;
      margin-bottom: 6px;
      color: #e5e7eb;
    }
    .form-label .req { color: var(--danger); margin-left: 3px; }
    .form-label .desc { font-weight: normal; font-size: 12px; color: var(--text-muted); margin-left: 6px; }
    .form-control {
      width: 100%;
      background: var(--input-bg);
      border: 1px solid var(--input-border);
      border-radius: 6px;
      padding: 8px 12px;
      color: var(--text);
      font-size: 14px;
      font-family: inherit;
      outline: none;
      transition: border-color 0.15s ease;
    }
    .form-control:focus { border-color: var(--primary); }
    textarea.form-control { min-height: 80px; font-family: var(--font-mono); font-size: 13px; resize: vertical; }

    /* Array container */
    .array-container {
      background: #182234;
      border: 1px solid #293548;
      border-radius: 8px;
      padding: 12px;
      margin-top: 6px;
    }
    .array-item {
      display: flex;
      gap: 8px;
      align-items: flex-start;
      margin-bottom: 8px;
    }
    .array-item-content { flex: 1; }
    .btn-icon {
      background: #374151;
      border: none;
      color: #f87171;
      padding: 7px 10px;
      border-radius: 6px;
      cursor: pointer;
      font-size: 12px;
      font-weight: bold;
    }
    .btn-icon:hover { background: #4b5563; }
    .btn-add {
      background: #1f2937;
      border: 1px dashed #4b5563;
      color: #60a5fa;
      padding: 6px 12px;
      border-radius: 6px;
      cursor: pointer;
      font-size: 13px;
      font-weight: 500;
      width: 100%;
      text-align: center;
      transition: all 0.15s ease;
    }
    .btn-add:hover { border-color: #60a5fa; background: #263346; }

    /* Nested Object container */
    .object-container {
      background: #151e2e;
      border: 1px solid #233045;
      border-radius: 8px;
      padding: 14px;
      margin-top: 6px;
    }

    /* Actions */
    .actions { display: flex; gap: 12px; margin-top: 20px; align-items: center; }
    .btn-primary {
      background: var(--primary);
      border: none;
      color: white;
      padding: 10px 20px;
      border-radius: 6px;
      font-size: 14px;
      font-weight: 600;
      cursor: pointer;
      transition: background 0.15s ease;
    }
    .btn-primary:hover { background: var(--primary-hover); }
    .btn-secondary {
      background: #374151;
      border: none;
      color: #e5e7eb;
      padding: 10px 16px;
      border-radius: 6px;
      font-size: 14px;
      font-weight: 500;
      cursor: pointer;
    }
    .btn-secondary:hover { background: #4b5563; }

    /* Response Panel */
    .response-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 12px;
      min-height: 36px;
    }
    .status-badge {
      font-size: 14px;
      font-weight: 700;
      padding: 4px 10px;
      border-radius: 6px;
      font-family: var(--font-mono);
    }
    .status-2xx { background: #064e3b; color: #34d399; }
    .status-3xx { background: #1e3a8a; color: #60a5fa; }
    .status-4xx { background: #78350f; color: #fbbf24; }
    .status-5xx { background: #7f1d1d; color: #f87171; }
    .time-badge { font-size: 13px; color: var(--text-muted); font-family: var(--font-mono); }

    /* Code View */
    pre.code-view {
      background: #090d16;
      border: 1px solid #1f2937;
      border-radius: 8px;
      padding: 14px;
      color: #e2e8f0;
      font-family: var(--font-mono);
      font-size: 13px;
      overflow-x: auto;
      max-height: 480px;
      white-space: pre-wrap;
      word-break: break-all;
    }
    iframe.preview-frame {
      width: 100%;
      height: 400px;
      background: white;
      border: 1px solid var(--card-border);
      border-radius: 8px;
    }
    .headers-table {
      width: 100%;
      font-size: 13px;
      font-family: var(--font-mono);
      border-collapse: collapse;
    }
    .headers-table td { padding: 6px 8px; border-bottom: 1px solid #1f2937; }
    .headers-table td.header-key { color: #93c5fd; font-weight: 600; width: 35%; }
  </style>
</head>
<body>
  <div class="container">
    <!-- Top Bar -->
    <div class="topbar">
      <div class="brand">
        <span class="brand-logo">NXP</span>
        <span class="brand-title">API Explorer &amp; Contract Tester</span>
      </div>
      <div class="format-pills">
        <a href="` + html.EscapeString(view.Path) + `?nxp=json" class="pill">JSON (?nxp=json)</a>
        <a href="` + html.EscapeString(view.Path) + `?nxp=html" class="pill active">HTML (?nxp=html)</a>
        <a href="` + html.EscapeString(view.Path) + `?nxp=text" class="pill">Tree (?nxp=text)</a>
        <a href="` + html.EscapeString(view.Path) + `?nxp=openapi" class="pill">OpenAPI</a>
      </div>
    </div>

    <!-- Path Metadata Card -->
    <div class="card">
      <div class="path-row">
        ` + renderBreadcrumbsHTML(view.Path))

	if view.Self != nil {
		for _, m := range view.Self.Methods {
			cls := "badge"
			switch strings.ToUpper(m) {
			case "GET":
				cls += " badge-get"
			case "POST":
				cls += " badge-post"
			case "PUT":
				cls += " badge-put"
			case "DELETE":
				cls += " badge-delete"
			case "PATCH":
				cls += " badge-patch"
			}
			sb.WriteString(fmt.Sprintf(`<span class="%s">%s</span>`, cls, html.EscapeString(m)))
		}
		if view.Self.Auth != "" {
			sb.WriteString(fmt.Sprintf(`<span class="badge badge-auth">auth: %s</span>`, html.EscapeString(view.Self.Auth)))
		}
		if len(view.Self.Scopes) > 0 {
			sb.WriteString(fmt.Sprintf(`<span class="meta-text">scopes: [%s]</span>`, html.EscapeString(strings.Join(view.Self.Scopes, ", "))))
		}
		if view.Self.RateLimit != nil {
			sb.WriteString(fmt.Sprintf(`<span class="meta-text">ratelimit: %drps (burst %d)</span>`, view.Self.RateLimit.RPS, view.Self.RateLimit.Burst))
		}
	} else {
		sb.WriteString(`<span class="meta-text">(Directory path - no handler directly at this level)</span>`)
	}

	sb.WriteString(`</div>`)

	// Children sub-routes
	if len(view.Children) > 0 {
		sb.WriteString(`<div class="meta-text">Sub-Routes:</div><div class="children-nav">`)
		for _, c := range view.Children {
			target := view.Path
			if target == "/" {
				target = ""
			}
			target = target + "/" + c.Name + "?nxp=html"
			sb.WriteString(fmt.Sprintf(`<a href="%s" class="child-link">%s`, html.EscapeString(target), html.EscapeString(c.Name)))
			if c.Kind == "branch" {
				sb.WriteString("/")
			}
			for _, m := range c.Methods {
				sb.WriteString(fmt.Sprintf(` <span style="font-size:10px;opacity:0.8;">%s</span>`, m))
			}
			sb.WriteString(`</a>`)
		}
		sb.WriteString(`</div>`)
	}

	sb.WriteString(`</div>

    <!-- Main Tabs -->
    <div class="card">
      <div class="tabs">
        <button class="tab-btn active" onclick="switchMainTab('tester')">Interactive Tester</button>
        <button class="tab-btn" onclick="switchMainTab('contract')">Contract Schema (JSON)</button>
      </div>

      <!-- Tab: Interactive Tester -->
      <div id="tab-tester" class="tab-content active">
        <div class="split-layout">
          <!-- Left: Request Builder -->
          <div>
            <h3 style="margin-bottom:14px; font-size:16px;">Request Configuration</h3>
            
            <div style="display:flex; gap:10px; margin-bottom:14px;">
              <select id="req-method" class="form-control" style="width:120px; font-weight:700; font-family:var(--font-mono);" onchange="buildFormFromSchema()">`)

	// Populate methods
	if view.Self != nil && len(view.Self.Methods) > 0 {
		for _, m := range view.Self.Methods {
			sb.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, m, m))
		}
	} else {
		for _, m := range []string{"GET", "POST", "PUT", "DELETE", "PATCH"} {
			sb.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, m, m))
		}
	}

	sb.WriteString(`</select>
              <input id="req-url" type="text" class="form-control" value="` + html.EscapeString(view.Path) + `" style="font-family:var(--font-mono);">
            </div>

            <!-- Request Body Mode Switcher -->
            <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:10px;">
              <span style="font-size:14px; font-weight:600;">Request Body</span>
              <div style="display:flex; gap:6px;">
                <button type="button" id="btn-mode-form" class="pill active" style="padding:3px 10px; font-size:12px;" onclick="setBodyMode('form')">Form Builder</button>
                <button type="button" id="btn-mode-json" class="pill" style="padding:3px 10px; font-size:12px;" onclick="setBodyMode('json')">Raw JSON</button>
              </div>
            </div>

            <!-- Form Builder Container -->
            <div id="body-form-view">
              <div id="schema-form-root"></div>
            </div>

            <!-- Raw JSON View -->
            <div id="body-json-view" style="display:none;">
              <textarea id="raw-json-editor" class="form-control" style="height:240px;"></textarea>
            </div>

            <div class="actions">
              <button type="button" id="btn-send" class="btn-primary" onclick="sendRequest()">&#9654; Send Request</button>
              <button type="button" class="btn-secondary" onclick="resetForm()">Reset Form</button>
            </div>
          </div>

          <!-- Right: Response Inspector -->
          <div>
            <h3 style="margin-bottom:14px; font-size:16px;">Response</h3>
            <div class="response-header">
              <div id="res-status-container">
                <span style="font-size:13px; color:var(--text-muted);">Ready to test</span>
              </div>
              <div id="res-time-container"></div>
            </div>

            <div class="tabs" style="margin-bottom:12px;">
              <button id="res-tab-preview" class="tab-btn active" onclick="switchResTab('preview')">Preview</button>
              <button id="res-tab-raw" class="tab-btn" onclick="switchResTab('raw')">Raw Body</button>
              <button id="res-tab-headers" class="tab-btn" onclick="switchResTab('headers')">Headers</button>
            </div>

            <div id="res-content-preview">
              <div id="res-preview-empty" style="padding:30px; text-align:center; color:var(--text-muted); border:1px dashed var(--input-border); border-radius:8px;">
                Click "Send Request" to test this endpoint
              </div>
              <div id="res-preview-active" style="display:none;"></div>
            </div>

            <div id="res-content-raw" style="display:none;">
              <pre id="res-raw-text" class="code-view"></pre>
            </div>

            <div id="res-content-headers" style="display:none;">
              <table id="res-headers-table" class="headers-table"></table>
            </div>
          </div>
        </div>
      </div>

      <!-- Tab: Contract Schema (JSON) -->
      <div id="tab-contract" class="tab-content">
        <h3 style="margin-bottom:10px; font-size:16px;">Contract Schema</h3>
        <pre class="code-view" id="contract-json-view"></pre>
      </div>
    </div>
  </div>

  <!-- Raw Embedded View Data -->
  <script id="nxp-data" type="application/json">` + safeJSON + `</script>

  <script>
    const nxpData = JSON.parse(document.getElementById('nxp-data').textContent);
    let currentBodyMode = 'form';
    let formDataModel = {};

    // Initial load
    document.addEventListener('DOMContentLoaded', () => {
      document.getElementById('contract-json-view').textContent = JSON.stringify(nxpData, null, 2);
      buildFormFromSchema();
    });

    function switchMainTab(tab) {
      document.querySelectorAll('.tabs .tab-btn').forEach(b => b.classList.remove('active'));
      document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
      if (tab === 'tester') {
        document.querySelector('.tabs .tab-btn:nth-child(1)').classList.add('active');
        document.getElementById('tab-tester').classList.add('active');
      } else {
        document.querySelector('.tabs .tab-btn:nth-child(2)').classList.add('active');
        document.getElementById('tab-contract').classList.add('active');
      }
    }

    function switchResTab(tab) {
      document.getElementById('res-tab-preview').classList.toggle('active', tab === 'preview');
      document.getElementById('res-tab-raw').classList.toggle('active', tab === 'raw');
      document.getElementById('res-tab-headers').classList.toggle('active', tab === 'headers');
      document.getElementById('res-content-preview').style.display = tab === 'preview' ? 'block' : 'none';
      document.getElementById('res-content-raw').style.display = tab === 'raw' ? 'block' : 'none';
      document.getElementById('res-content-headers').style.display = tab === 'headers' ? 'block' : 'none';
    }

    function setBodyMode(mode) {
      currentBodyMode = mode;
      document.getElementById('btn-mode-form').classList.toggle('active', mode === 'form');
      document.getElementById('btn-mode-json').classList.toggle('active', mode === 'json');
      document.getElementById('body-form-view').style.display = mode === 'form' ? 'block' : 'none';
      document.getElementById('body-json-view').style.display = mode === 'json' ? 'block' : 'none';
      if (mode === 'json') {
        document.getElementById('raw-json-editor').value = JSON.stringify(formDataModel, null, 2);
      } else {
        try {
          const parsed = JSON.parse(document.getElementById('raw-json-editor').value);
          formDataModel = parsed;
          buildFormFromSchema(formDataModel);
        } catch(e) {}
      }
    }

    // Resolve request schema from view for the currently selected method.
    // The schema contract is keyed by method: GET and POST on the same path
    // carry different request/response shapes.
    function getRequestSchema() {
      if (!nxpData || !nxpData.self) return null;
      const schemas = nxpData.self.schemas || {};
      const methodEl = document.getElementById('req-method');
      const method = methodEl ? methodEl.value : (nxpData.self.methods && nxpData.self.methods[0]);
      const s = schemas[method];
      if (!s) return null;
      return s.request || s.Request || s.in || s.In || s.body || s.Body || (s.type ? s : null);
    }

    // Dynamic Form Builder
    function buildFormFromSchema(initialValues) {
      const root = document.getElementById('schema-form-root');
      root.innerHTML = '';
      const reqSchema = getRequestSchema();

      if (!reqSchema || (reqSchema.type !== 'object' && !reqSchema.properties)) {
        root.innerHTML = '<div style="padding:16px; color:var(--text-muted); font-size:13px; background:#162030; border-radius:6px;">No specific body schema required for this route. You can switch to Raw JSON to send custom payloads.</div>';
        formDataModel = initialValues || {};
        return;
      }

      formDataModel = initialValues || generateDefaultData(reqSchema);
      renderObjectForm(root, reqSchema, formDataModel, '');
      document.getElementById('raw-json-editor').value = JSON.stringify(formDataModel, null, 2);
    }

    function generateDefaultData(schema) {
      if (!schema) return null;
      if (schema.enum && schema.enum.length > 0) return schema.enum[0];
      if (schema.type === 'string') return '';
      if (schema.type === 'integer' || schema.type === 'number') return schema.minimum !== undefined ? schema.minimum : 0;
      if (schema.type === 'boolean') return false;
      if (schema.type === 'array') return [];
      if (schema.type === 'object' || schema.properties) {
        const obj = {};
        if (schema.properties) {
          for (const key of Object.keys(schema.properties)) {
            obj[key] = generateDefaultData(schema.properties[key]);
          }
        }
        return obj;
      }
      return null;
    }

    function renderObjectForm(parent, schema, dataObj, pathPrefix) {
      const properties = schema.properties || {};
      const required = schema.required || [];

      for (const [propName, propSchema] of Object.entries(properties)) {
        const fullPath = pathPrefix ? pathPrefix + '.' + propName : propName;
        const isReq = required.includes(propName);
        const formGroup = document.createElement('div');
        formGroup.className = 'form-group';

        const label = document.createElement('label');
        label.className = 'form-label';
        label.innerHTML = propName + (isReq ? '<span class="req">*</span>' : '') + (propSchema.description ? '<span class="desc">' + escapeHtml(propSchema.description) + '</span>' : '');
        formGroup.appendChild(label);

        // 1. Enum / Options Dropdown
        if (propSchema.enum && propSchema.enum.length > 0) {
          const select = document.createElement('select');
          select.className = 'form-control';
          for (const opt of propSchema.enum) {
            const optEl = document.createElement('option');
            optEl.value = opt;
            optEl.textContent = opt;
            if (dataObj[propName] === opt) optEl.selected = true;
            select.appendChild(optEl);
          }
          select.onchange = (e) => {
            dataObj[propName] = e.target.value;
            syncJsonEditor();
          };
          formGroup.appendChild(select);
        }
        // 2. Boolean
        else if (propSchema.type === 'boolean') {
          const select = document.createElement('select');
          select.className = 'form-control';
          select.innerHTML = '<option value="false">false</option><option value="true">true</option>';
          select.value = dataObj[propName] ? 'true' : 'false';
          select.onchange = (e) => {
            dataObj[propName] = e.target.value === 'true';
            syncJsonEditor();
          };
          formGroup.appendChild(select);
        }
        // 3. Number / Integer
        else if (propSchema.type === 'integer' || propSchema.type === 'number') {
          const input = document.createElement('input');
          input.type = 'number';
          input.className = 'form-control';
          if (propSchema.type === 'integer') input.step = '1';
          if (propSchema.minimum !== undefined) input.min = propSchema.minimum;
          if (propSchema.maximum !== undefined) input.max = propSchema.maximum;
          input.value = dataObj[propName] !== undefined ? dataObj[propName] : 0;
          input.oninput = (e) => {
            dataObj[propName] = parseFloat(e.target.value) || 0;
            syncJsonEditor();
          };
          formGroup.appendChild(input);
        }
        // 4. Array / List
        else if (propSchema.type === 'array') {
          if (!Array.isArray(dataObj[propName])) dataObj[propName] = [];
          const arrContainer = document.createElement('div');
          arrContainer.className = 'array-container';
          
          const itemsList = document.createElement('div');
          renderArrayItems(itemsList, propSchema.items || { type: 'string' }, dataObj[propName]);
          arrContainer.appendChild(itemsList);

          const addBtn = document.createElement('button');
          addBtn.type = 'button';
          addBtn.className = 'btn-add';
          addBtn.textContent = '+ Add Item to ' + propName;
          addBtn.onclick = () => {
            const defItem = generateDefaultData(propSchema.items || { type: 'string' });
            dataObj[propName].push(defItem);
            renderArrayItems(itemsList, propSchema.items || { type: 'string' }, dataObj[propName]);
            syncJsonEditor();
          };
          arrContainer.appendChild(addBtn);
          formGroup.appendChild(arrContainer);
        }
        // 5. Nested Object
        else if (propSchema.type === 'object' || propSchema.properties) {
          if (!dataObj[propName] || typeof dataObj[propName] !== 'object') {
            dataObj[propName] = {};
          }
          const nestedContainer = document.createElement('div');
          nestedContainer.className = 'object-container';
          renderObjectForm(nestedContainer, propSchema, dataObj[propName], fullPath);
          formGroup.appendChild(nestedContainer);
        }
        // 6. String / Default
        else {
          const input = document.createElement('input');
          input.className = 'form-control';
          if (propSchema.format === 'email') input.type = 'email';
          else if (propSchema.format === 'date-time') input.type = 'datetime-local';
          else if (propName.toLowerCase().includes('password')) input.type = 'password';
          else input.type = 'text';

          input.value = dataObj[propName] || '';
          input.oninput = (e) => {
            dataObj[propName] = e.target.value;
            syncJsonEditor();
          };
          formGroup.appendChild(input);
        }

        parent.appendChild(formGroup);
      }
    }

    function renderArrayItems(container, itemSchema, arr) {
      container.innerHTML = '';
      arr.forEach((val, idx) => {
        const itemRow = document.createElement('div');
        itemRow.className = 'array-item';

        const content = document.createElement('div');
        content.className = 'array-item-content';

        if (itemSchema.type === 'object' || itemSchema.properties) {
          renderObjectForm(content, itemSchema, arr[idx], '[' + idx + ']');
        } else {
          const inp = document.createElement('input');
          inp.className = 'form-control';
          inp.type = (itemSchema.type === 'integer' || itemSchema.type === 'number') ? 'number' : 'text';
          inp.value = val !== undefined ? val : '';
          inp.oninput = (e) => {
            arr[idx] = (itemSchema.type === 'integer' || itemSchema.type === 'number') ? parseFloat(e.target.value) || 0 : e.target.value;
            syncJsonEditor();
          };
          content.appendChild(inp);
        }

        const delBtn = document.createElement('button');
        delBtn.type = 'button';
        delBtn.className = 'btn-icon';
        delBtn.textContent = '✕';
        delBtn.onclick = () => {
          arr.splice(idx, 1);
          renderArrayItems(container, itemSchema, arr);
          syncJsonEditor();
        };

        itemRow.appendChild(content);
        itemRow.appendChild(delBtn);
        container.appendChild(itemRow);
      });
    }

    function syncJsonEditor() {
      document.getElementById('raw-json-editor').value = JSON.stringify(formDataModel, null, 2);
    }

    function resetForm() {
      buildFormFromSchema();
    }

    // Send Request Handler
    async function sendRequest() {
      const btn = document.getElementById('btn-send');
      const method = document.getElementById('req-method').value;
      const url = document.getElementById('req-url').value;

      const headers = {
        'Content-Type': 'application/json'
      };

      const options = {
        method,
        headers,
        credentials: 'same-origin'
      };

      // Attach body if method allows
      if (method !== 'GET' && method !== 'HEAD') {
        if (currentBodyMode === 'json') {
          options.body = document.getElementById('raw-json-editor').value;
        } else {
          options.body = JSON.stringify(formDataModel);
        }
      }

      btn.disabled = true;
      btn.textContent = 'Sending...';

      const t0 = performance.now();
      try {
        const resp = await fetch(url, options);
        const t1 = performance.now();
        const duration = (t1 - t0).toFixed(1);

        displayResponse(resp, duration);
      } catch (err) {
        const t1 = performance.now();
        displayError(err, (t1 - t0).toFixed(1));
      } finally {
        btn.disabled = false;
        btn.textContent = '▶ Send Request';
      }
    }

    async function displayResponse(resp, durationMs) {
      // 1. Status badge
      const statusContainer = document.getElementById('res-status-container');
      let statusCls = 'status-2xx';
      if (resp.status >= 300 && resp.status < 400) statusCls = 'status-3xx';
      else if (resp.status >= 400 && resp.status < 500) statusCls = 'status-4xx';
      else if (resp.status >= 500) statusCls = 'status-5xx';

      statusContainer.innerHTML = '<span class="status-badge ' + statusCls + '">' + resp.status + ' ' + (resp.statusText || '') + '</span>';
      document.getElementById('res-time-container').innerHTML = '<span class="time-badge">' + durationMs + ' ms</span>';

      // 2. Headers
      const headersTable = document.getElementById('res-headers-table');
      headersTable.innerHTML = '';
      resp.headers.forEach((v, k) => {
        const tr = document.createElement('tr');
        tr.innerHTML = '<td class="header-key">' + escapeHtml(k) + '</td><td>' + escapeHtml(v) + '</td>';
        headersTable.appendChild(tr);
      });

      // 3. Body
      const contentType = (resp.headers.get('content-type') || '').toLowerCase();
      const rawText = await resp.text();
      document.getElementById('res-raw-text').textContent = rawText;

      const previewContainer = document.getElementById('res-preview-active');
      document.getElementById('res-preview-empty').style.display = 'none';
      previewContainer.style.display = 'block';

      if (contentType.includes('json')) {
        try {
          const parsed = JSON.parse(rawText);
          previewContainer.innerHTML = '<pre class="code-view">' + escapeHtml(JSON.stringify(parsed, null, 2)) + '</pre>';
        } catch(e) {
          previewContainer.innerHTML = '<pre class="code-view">' + escapeHtml(rawText) + '</pre>';
        }
      } else if (contentType.includes('html')) {
        previewContainer.innerHTML = '<div style="margin-bottom:8px; font-size:12px; color:var(--text-muted);">Rendered HTML Preview:</div><iframe class="preview-frame" sandbox="allow-same-origin" srcdoc="' + escapeHtmlAttr(rawText) + '"></iframe>';
      } else if (contentType.includes('image/svg')) {
        previewContainer.innerHTML = '<div style="background:#1e293b; padding:16px; border-radius:8px; text-align:center;">' + rawText + '</div>';
      } else {
        previewContainer.innerHTML = '<pre class="code-view">' + escapeHtml(rawText) + '</pre>';
      }
    }

    function displayError(err, durationMs) {
      document.getElementById('res-status-container').innerHTML = '<span class="status-badge status-5xx">Network Error</span>';
      document.getElementById('res-time-container').innerHTML = '<span class="time-badge">' + durationMs + ' ms</span>';
      document.getElementById('res-preview-empty').style.display = 'none';
      const previewContainer = document.getElementById('res-preview-active');
      previewContainer.style.display = 'block';
      previewContainer.innerHTML = '<div style="color:var(--danger); padding:16px; background:#291415; border-radius:8px;">' + escapeHtml(err.message || 'Failed to fetch') + '</div>';
    }

    function escapeHtml(str) {
      if (!str) return '';
      return String(str).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
    }
    function escapeHtmlAttr(str) {
      if (!str) return '';
      return String(str).replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    }
  </script>
</body>
</html>`)

	return sb.String()
}

// renderBreadcrumbsHTML renders breadcrumb links for path hierarchy, e.g. /foo/bar/test -> [/] / [foo] / [bar] / [test]
func renderBreadcrumbsHTML(path string) string {
	clean := strings.TrimSpace(path)
	if clean == "" || clean == "/" {
		return `<nav class="breadcrumbs"><span class="breadcrumb-item current">/</span></nav>`
	}

	parts := strings.Split(strings.Trim(clean, "/"), "/")
	var sb strings.Builder
	sb.WriteString(`<nav class="breadcrumbs">`)
	sb.WriteString(`<a href="/?nxp=html" class="breadcrumb-item" title="Root /">/</a>`)

	accum := ""
	for i, part := range parts {
		if part == "" {
			continue
		}
		accum += "/" + part
		sb.WriteString(`<span class="breadcrumb-sep">/</span>`)
		if i == len(parts)-1 {
			sb.WriteString(fmt.Sprintf(`<span class="breadcrumb-item current">%s</span>`, html.EscapeString(part)))
		} else {
			sb.WriteString(fmt.Sprintf(`<a href="%s?nxp=html" class="breadcrumb-item">%s</a>`, html.EscapeString(accum), html.EscapeString(part)))
		}
	}
	sb.WriteString(`</nav>`)
	return sb.String()
}
