"""
Tests for web/app.js — validates renderRunCard variants, toggleLog, copyLog,
and removal of toggleRunLog.
"""
import re
import pytest
from pathlib import Path

APP_JS_PATH = Path(__file__).parent.parent.parent / "web" / "app.js"


@pytest.fixture(scope="module")
def js_text():
    return APP_JS_PATH.read_text()


# ---------------------------------------------------------------------------
# renderRunCard — running variant
# ---------------------------------------------------------------------------

class TestRenderRunCardRunning:
    def test_running_card_has_live_class(self, js_text):
        """Running card must use class 'run-card live'."""
        assert 'run-card live' in js_text, \
            "renderRunCard must produce a card with class 'run-card live' for running status"

    def test_running_card_id(self, js_text):
        """Running card must have id='run-${run.id}'."""
        assert 'id="run-${run.id}"' in js_text or "id=`run-${run.id}`" in js_text, \
            "Running card must have id set to run-${run.id}"

    def test_running_card_status_icon_id(self, js_text):
        """Running card status icon must have id='status-icon-${run.id}'."""
        assert 'status-icon-${run.id}' in js_text, \
            "Running card must have status icon with id status-icon-${run.id}"

    def test_running_card_status_icon_blue_color(self, js_text):
        """Running card status icon must use blue color #2196F3."""
        assert '#2196F3' in js_text, \
            "Running card status icon must use blue color #2196F3"

    def test_running_card_run_time_id(self, js_text):
        """Running card run-time element must have id='run-time-${run.id}'."""
        assert 'run-time-${run.id}' in js_text, \
            "Running card must have run-time element with id run-time-${run.id}"

    def test_running_card_started_text(self, js_text):
        """Running card run-time must show 'started ...'."""
        assert 'started ${timeAgo(' in js_text or "started ${ timeAgo(" in js_text, \
            "Running card run-time must show 'started ' + timeAgo(run.startedAt)"

    def test_running_card_live_badge_id(self, js_text):
        """Running card must have live-badge with id='live-badge-${run.id}'."""
        assert 'live-badge-${run.id}' in js_text, \
            "Running card must have live-badge with id live-badge-${run.id}"

    def test_running_card_live_badge_text(self, js_text):
        """Running card live-badge must show '● live'."""
        assert '● live' in js_text or '\u25cf live' in js_text, \
            "Running card live-badge must display '● live'"

    def test_running_card_log_toggle_calls_toggleLog(self, js_text):
        """Running card log-toggle must call toggleLog."""
        assert "toggleLog('${run.id}'" in js_text or 'toggleLog(`${run.id}`' in js_text or "toggleLog('${run.id}'," in js_text, \
            "Running card log-toggle must call toggleLog with run.id"

    def test_running_card_log_toggle_text_hide(self, js_text):
        """Running card log-toggle initial text must be 'hide ▴'."""
        assert 'hide \u25b4' in js_text, \
            "Running card log-toggle must have initial text 'hide ▴'"

    def test_running_card_log_panel_not_hidden(self, js_text):
        """Running card log panel must NOT have class 'hidden' initially."""
        # The running card log panel should use id log-${run.id} without 'hidden' class
        # We verify that the log-panel for running status does not include 'hidden'
        # Find the running branch of renderRunCard
        running_branch = _extract_running_branch(js_text)
        assert running_branch is not None, "Could not extract running card branch from renderRunCard"
        assert 'log-panel hidden' not in running_branch, \
            "Running card log panel must NOT have 'hidden' class"
        assert 'log-panel' in running_branch, \
            "Running card must have a log-panel"

    def test_running_card_log_panel_id(self, js_text):
        """Running card log panel must have id='log-${run.id}'."""
        assert 'log-${run.id}' in js_text, \
            "Running card must have log panel with id log-${run.id}"

    def test_running_card_log_label_streaming(self, js_text):
        """Running card log toolbar label must say 'streaming output'."""
        assert 'streaming output' in js_text, \
            "Running card log toolbar must show 'streaming output'"

    def test_running_card_log_label_id(self, js_text):
        """Running card log label must have id='log-label-${run.id}'."""
        assert 'log-label-${run.id}' in js_text, \
            "Running card must have log label with id log-label-${run.id}"

    def test_running_card_copy_button_calls_copyLog(self, js_text):
        """Running card copy button must call copyLog."""
        assert "copyLog('${run.id}')" in js_text or 'copyLog(`${run.id}`)' in js_text, \
            "Running card must have a copy button calling copyLog"

    def test_running_card_log_output_id(self, js_text):
        """Running card log pre must have id='logout-${run.id}'."""
        assert 'logout-${run.id}' in js_text, \
            "Running card log pre must have id logout-${run.id}"

    def test_running_card_cursor_span_id(self, js_text):
        """Running card must have cursor span with id='cursor-${run.id}'."""
        assert 'cursor-${run.id}' in js_text, \
            "Running card must have cursor span with id cursor-${run.id}"

    def test_running_card_cursor_span_class(self, js_text):
        """Running card must have cursor span with class='log-cursor'."""
        assert 'log-cursor' in js_text, \
            "Running card cursor span must have class log-cursor"

    def test_running_card_cursor_char(self, js_text):
        """Running card cursor must contain the block cursor character ▌."""
        assert '\u258c' in js_text, \
            "Running card cursor span must contain '▌' character"


# ---------------------------------------------------------------------------
# renderRunCard — completed/success variant
# ---------------------------------------------------------------------------

class TestRenderRunCardCompleted:
    def test_success_icon(self, js_text):
        """Success card must use ✓ icon."""
        assert '\u2713' in js_text, "renderRunCard must use ✓ icon for success"

    def test_success_color_green(self, js_text):
        """Success card must use green color."""
        assert 'var(--green)' in js_text, "Success card must use var(--green) color"

    def test_completed_log_panel_hidden(self, js_text):
        """Completed card log panel must have class 'hidden'."""
        assert 'log-panel hidden' in js_text, \
            "Completed/failed card log panel must have class 'hidden'"

    def test_completed_log_toggle_text_logs(self, js_text):
        """Completed card log-toggle initial text must be 'logs ▾'."""
        assert 'logs \u25be' in js_text, \
            "Completed card log-toggle must have text 'logs ▾'"

    def test_completed_duration_shown(self, js_text):
        """Completed card run-time must show duration via durationMs."""
        assert 'durationMs(' in js_text, \
            "Completed card must show duration using durationMs()"

    def test_completed_log_output_has_run_output(self, js_text):
        """Completed card log pre must contain esc(run.output)."""
        assert "esc(run.output" in js_text, \
            "Completed card log pre must contain esc(run.output || '')"


# ---------------------------------------------------------------------------
# renderRunCard — failed variant
# ---------------------------------------------------------------------------

class TestRenderRunCardFailed:
    def test_failed_icon(self, js_text):
        """Failed card must use ✕ icon."""
        assert '\u2715' in js_text, "renderRunCard must use ✕ icon for failure"

    def test_failed_color_red(self, js_text):
        """Failed card must use red color."""
        assert 'var(--red)' in js_text, "Failed card must use var(--red) color"

    def test_failed_class_applied(self, js_text):
        """Failed card must have 'run-failed' class."""
        assert 'run-failed' in js_text, \
            "renderRunCard must apply 'run-failed' class for non-success runs"


# ---------------------------------------------------------------------------
# toggleLog
# ---------------------------------------------------------------------------

class TestToggleLog:
    def test_toggleLog_exists(self, js_text):
        """toggleLog function must exist."""
        assert re.search(r'function\s+toggleLog\s*\(', js_text), \
            "toggleLog function must be defined"

    def test_toggleLog_gets_panel_by_log_id(self, js_text):
        """toggleLog must look up element by 'log-' + id."""
        assert re.search(r'getElementById\s*\(\s*[`\'"]log-\$\{id\}', js_text), \
            "toggleLog must get panel by id 'log-${id}'"

    def test_toggleLog_toggles_hidden(self, js_text):
        """toggleLog must toggle 'hidden' class on the panel."""
        assert re.search(r'classList\.toggle\s*\(\s*[\'"]hidden[\'"]\s*\)', js_text), \
            "toggleLog must use classList.toggle('hidden')"

    def test_toggleLog_sets_text_logs_when_hidden(self, js_text):
        """toggleLog must set link text to 'logs ▾' when panel is hidden."""
        assert 'logs \u25be' in js_text, \
            "toggleLog must use 'logs ▾' text when panel is hidden"

    def test_toggleLog_sets_text_hide_when_visible(self, js_text):
        """toggleLog must set link text to 'hide ▴' when panel is visible."""
        assert 'hide \u25b4' in js_text, \
            "toggleLog must use 'hide ▴' text when panel is visible"


# ---------------------------------------------------------------------------
# copyLog
# ---------------------------------------------------------------------------

class TestCopyLog:
    def test_copyLog_exists(self, js_text):
        """copyLog function must exist."""
        assert re.search(r'function\s+copyLog\s*\(', js_text), \
            "copyLog function must be defined"

    def test_copyLog_gets_pre_by_logout_id(self, js_text):
        """copyLog must look up pre by 'logout-' + runId."""
        assert re.search(r'getElementById\s*\(\s*[`\'"]logout-\$\{runId\}', js_text), \
            "copyLog must get pre by id 'logout-${runId}'"

    def test_copyLog_filters_cursor_by_id(self, js_text):
        """copyLog must filter out the cursor span by checking n.id against cursor-${runId}."""
        assert 'cursor-${runId}' in js_text, \
            "copyLog must filter nodes by checking n.id !== 'cursor-${runId}'"

    def test_copyLog_uses_childNodes(self, js_text):
        """copyLog must iterate over childNodes."""
        assert 'childNodes' in js_text, \
            "copyLog must collect text from childNodes"

    def test_copyLog_uses_clipboard(self, js_text):
        """copyLog must use navigator.clipboard.writeText."""
        assert 'navigator.clipboard.writeText' in js_text, \
            "copyLog must use navigator.clipboard.writeText"

    def test_copyLog_silent_catch(self, js_text):
        """copyLog must silently catch clipboard errors with .catch(() => {})."""
        assert re.search(r'\.catch\s*\(\s*\(\s*\)\s*=>\s*\{\s*\}\s*\)', js_text), \
            "copyLog must silently catch clipboard errors"


# ---------------------------------------------------------------------------
# toggleRunLog — must be REMOVED
# ---------------------------------------------------------------------------

class TestToggleRunLogRemoved:
    def test_toggleRunLog_function_removed(self, js_text):
        """Old toggleRunLog function must be removed."""
        assert not re.search(r'function\s+toggleRunLog\s*\(', js_text), \
            "toggleRunLog must be removed (superseded by toggleLog)"

    def test_toggleRunLog_not_called(self, js_text):
        """toggleRunLog must not be called anywhere."""
        assert 'toggleRunLog' not in js_text, \
            "toggleRunLog must not appear anywhere in app.js"


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _extract_running_branch(js_text):
    """
    Try to extract the 'running' branch of renderRunCard.
    Returns the substring between 'run.status === .running.' and the first
    occurrence of 'run-card run-failed' or the end of the if block.
    """
    m = re.search(r"status\s*===\s*['\"]running['\"]", js_text)
    if not m:
        return None
    # Return a generous slice after the status check
    start = m.start()
    # Find closing of the if block — look for the else/next return or 200 chars
    snippet = js_text[start:start + 800]
    return snippet
