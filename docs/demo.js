// demo.js — WASM bridge and UI handling for tldt web demo

let wasmReady = false;
let go = null;

// Example text for demo
const EXAMPLE_TEXT = `The rapid advancement of artificial intelligence has fundamentally transformed how we approach software development. Large language models can now generate code, explain complex algorithms, and debug programs with remarkable accuracy. However, this technological revolution brings significant challenges that developers must navigate carefully.

First, there's the issue of code quality and maintainability. AI-generated code often works initially but may lack proper error handling, documentation, or adherence to project conventions. Developers must review AI suggestions critically, treating them as starting points rather than final solutions. The responsibility for code quality remains firmly with human developers.

Security presents another major concern. AI systems might inadvertently suggest code with vulnerabilities, especially when working with authentication, data validation, or cryptographic operations. Developers must apply the same security rigor to AI-assisted code as they would to any third-party library or copied snippet from Stack Overflow.

Intellectual property considerations also come into play. Training data for large language models includes vast amounts of open-source code, raising questions about license compliance when AI generates code similar to copyrighted implementations. Organizations need clear policies about AI tool usage and code provenance.

Despite these challenges, the productivity benefits are substantial. Developers report significant time savings on boilerplate code, documentation, and learning new APIs. The key is finding the right balance — using AI as an accelerator while maintaining human oversight for architecture decisions, critical logic, and quality assurance.

The future of software development likely involves deeper collaboration between humans and AI. Rather than replacing developers, these tools augment human capabilities, handling routine tasks while freeing developers to focus on creative problem-solving, system design, and understanding user needs. Success in this new paradigm requires adapting workflows, establishing best practices, and continuously learning how to work effectively with AI assistance.`;

// Load WASM on page load
async function initWasm() {
    const loadingEl = document.getElementById('wasmLoading');
    const readyEl = document.getElementById('wasmReady');
    const summarizeBtn = document.getElementById('summarizeBtn');

    try {
        const response = await fetch('tldt.wasm');
        if (!response.ok) {
            throw new Error('Failed to load tldt.wasm: ' + response.statusText);
        }

        const wasmBytes = await response.arrayBuffer();
        go = new Go();

        const result = await WebAssembly.instantiate(wasmBytes, go.importObject);
        go.run(result.instance);

        wasmReady = true;
        loadingEl.style.display = 'none';
        readyEl.style.display = 'block';
        summarizeBtn.disabled = false;
    } catch (err) {
        console.error('WASM init failed:', err);
        loadingEl.innerHTML = `<div class="error">❌ Failed to load WASM: ${err.message}</div>`;
    }
}

// Check URL parameters for bookmarklet or shared text
function checkURLParams() {
    const params = new URLSearchParams(window.location.search);
    const text = params.get('text');
    const auto = params.get('auto');

    if (text) {
        const decoded = decodeURIComponent(text);
        document.getElementById('inputText').value = decoded;

        if (auto === '1' && decoded.length > 100) {
            // Auto-summarize if loaded from bookmarklet
            setTimeout(runSummarize, 500);
        }
    }
}

// Load example text
function loadExample() {
    document.getElementById('inputText').value = EXAMPLE_TEXT;
}

// Clear input
function clearInput() {
    document.getElementById('inputText').value = '';
}

// Run summarization
function runSummarize() {
    if (!wasmReady) {
        alert('WASM not loaded yet. Please wait...');
        return;
    }

    const text = document.getElementById('inputText').value.trim();
    if (!text) {
        alert('Please enter some text to summarize.');
        return;
    }

    const btn = document.getElementById('summarizeBtn');
    const originalText = btn.innerHTML;
    btn.innerHTML = '<div class="spinner"></div> Processing...';
    btn.disabled = true;

    // Get settings
    const config = {
        text: text,
        algorithm: document.getElementById('algorithm').value,
        sentences: parseInt(document.getElementById('sentences').value),
        sanitize: document.getElementById('sanitize').checked,
        detectInjection: document.getElementById('detectInjection').checked,
        detectPII: document.getElementById('detectPII').checked,
        format: document.querySelector('input[name="format"]:checked').value,
        verbose: document.getElementById('verbose').checked
    };

    // Call WASM function
    try {
        const resultJSON = tldtSummarize(config);
        const result = JSON.parse(resultJSON);

        displayResult(result);
    } catch (err) {
        console.error('Summarize error:', err);
        document.getElementById('outputText').value = `Error: ${err.message}`;
    } finally {
        btn.innerHTML = originalText;
        btn.disabled = false;
    }
}

// Display result
function displayResult(result) {
    // Show error if any
    if (result.error) {
        document.getElementById('outputText').value = `Error: ${result.error}`;
        document.getElementById('copyBtn').disabled = true;
        hideMetrics();
        return;
    }

    // Output
    document.getElementById('outputText').value = result.rawOutput || result.summary;
    document.getElementById('copyBtn').disabled = false;

    // Detections
    const detectionsPanel = document.getElementById('detectionsPanel');
    const detectionsList = document.getElementById('detectionsList');

    if (result.detections && result.detections.length > 0) {
        detectionsList.innerHTML = result.detections.map(d => `
            <div class="detection ${d.type} ${d.severity}">
                <span class="detection-icon">${getDetectionIcon(d.type, d.severity)}</span>
                <span class="detection-message">${escapeHtml(d.message)}</span>
                <span class="detection-type">${d.type}</span>
            </div>
        `).join('');
        detectionsPanel.style.display = 'block';
    } else {
        detectionsPanel.style.display = 'none';
    }

    // Metrics
    if (result.metrics) {
        document.getElementById('metricSentences').textContent = result.metrics.sentenceCount;
        document.getElementById('metricInput').textContent = result.metrics.inputTokens.toLocaleString();
        document.getElementById('metricOutput').textContent = result.metrics.outputTokens.toLocaleString();
        document.getElementById('metricSavings').textContent = `${Math.round(result.metrics.savingsPercent)}%↓`;
        document.getElementById('metricAlgo').textContent = result.metrics.algorithm;
        document.getElementById('metricsBar').style.display = 'flex';
    } else {
        hideMetrics();
    }
}

function hideMetrics() {
    document.getElementById('metricsBar').style.display = 'none';
}

function getDetectionIcon(type, severity) {
    if (type === 'sanitized') return '✓';
    if (type === 'injection') return severity === 'high' ? '🚨' : '⚠️';
    if (type === 'pii') return '🔒';
    return 'ℹ️';
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Copy output
function copyOutput() {
    const output = document.getElementById('outputText');
    output.select();
    document.execCommand('copy');

    const btn = document.getElementById('copyBtn');
    const originalText = btn.innerHTML;
    btn.innerHTML = '✓ Copied!';
    setTimeout(() => {
        btn.innerHTML = originalText;
    }, 1500);
}

// Bookmarklet modal
function showBookmarkletModal() {
    document.getElementById('bookmarkletModal').classList.add('active');
}

function hideBookmarkletModal() {
    document.getElementById('bookmarkletModal').classList.remove('active');
}

// Close modal on overlay click
document.addEventListener('click', function(e) {
    const modal = document.getElementById('bookmarkletModal');
    if (e.target === modal) {
        hideBookmarkletModal();
    }
});

// Initialize on load
window.addEventListener('DOMContentLoaded', () => {
    initWasm();
    checkURLParams();
});

// Keyboard shortcuts
document.addEventListener('keydown', function(e) {
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
        runSummarize();
    }
});
