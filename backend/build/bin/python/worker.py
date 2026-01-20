#!/usr/bin/env python3
"""
Python Worker Process for ImageFlow
This script runs as a long-lived process that can execute multiple image processing tasks.
It reads commands from stdin and writes results to stdout using JSON.
"""

import sys
import json
import importlib
import traceback
import os
import subprocess
import gc
from typing import Dict, Any


SCRIPTS_DIR = os.path.dirname(os.path.abspath(__file__))
CONVERTER_TIMEOUT_S = int(os.getenv("IMAGEFLOW_CONVERTER_TIMEOUT_S", "180"))


def _run_converter_subprocess(input_data: Dict[str, Any]) -> Dict[str, Any]:
    script_path = os.path.join(SCRIPTS_DIR, "converter.py")
    try:
        popen_kwargs = {}
        if os.name == "nt":
            popen_kwargs["creationflags"] = getattr(subprocess, "CREATE_NO_WINDOW", 0)
        cp = subprocess.run(
            [sys.executable, script_path],
            input=json.dumps(input_data, ensure_ascii=False),
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
            timeout=CONVERTER_TIMEOUT_S,
            **popen_kwargs,
        )
    except subprocess.TimeoutExpired:
        return {"success": False, "error": f"converter timeout after {CONVERTER_TIMEOUT_S}s"}
    except Exception as e:
        return {"success": False, "error": f"failed to run converter subprocess: {e}"}

    stdout = (cp.stdout or "").strip()
    stderr = (cp.stderr or "").strip()
    if cp.returncode != 0:
        return {"success": False, "error": f"converter exit code {cp.returncode}", "stderr": stderr[-4000:]}
    if not stdout:
        return {"success": False, "error": "converter produced no output", "stderr": stderr[-4000:]}
    try:
        result = json.loads(stdout)
        if isinstance(result, dict):
            return result
        return {"success": False, "error": "converter returned non-object JSON"}
    except Exception as e:
        return {"success": False, "error": f"converter returned invalid JSON: {e}", "stdout": stdout[-4000:], "stderr": stderr[-4000:]}


def process_command(command: Dict[str, Any]) -> Dict[str, Any]:
    """
    Process a single command by dynamically loading and executing the appropriate script.

    Args:
        command: Dictionary with 'script' (script name) and 'input' (input data)

    Returns:
        Dictionary with processing result or error
    """
    try:
        script_name = command.get('script')
        input_data = command.get('input')

        if not script_name:
            return {'success': False, 'error': 'Missing script name'}

        if input_data is None:
            return {'success': False, 'error': 'Missing input data'}

        if script_name == "converter.py":
            return _run_converter_subprocess(input_data)

        # Import the module dynamically
        # Strip .py extension if present to get module name
        module_name = script_name[:-3] if script_name.endswith('.py') else script_name
        module = importlib.import_module(module_name)

        # Call the main processing function
        # Each script should have a process() function
        if hasattr(module, 'process'):
            result = module.process(input_data)
            return result
        else:
            return {'success': False, 'error': f'Script {script_name} has no process() function'}

    except ImportError as e:
        return {'success': False, 'error': f'Failed to import script: {str(e)}'}
    except Exception as e:
        return {
            'success': False,
            'error': f'Processing failed: {str(e)}',
            'traceback': traceback.format_exc()
        }


def main():
    """Main worker loop that processes commands from stdin."""
    try:
        sys.stdin.reconfigure(encoding="utf-8", errors="strict")
        sys.stdout.reconfigure(encoding="utf-8", errors="strict", line_buffering=True)
    except Exception:
        pass

    # Send ready signal
    print(json.dumps({'status': 'ready'}), flush=True)

    while True:
        try:
            # Read command from stdin
            line = sys.stdin.readline()

            if not line:
                # EOF reached, exit gracefully
                break

            # Parse command
            command = json.loads(line.strip())

            # Check for shutdown command
            if command.get('command') == 'shutdown':
                print(json.dumps({'status': 'shutdown'}), flush=True)
                break

            # Process the command
            result = process_command(command)

            # Write result to stdout
            print(json.dumps(result), flush=True)

            # Force garbage collection
            gc.collect()

        except json.JSONDecodeError as e:
            error_result = {'success': False, 'error': f'Invalid JSON: {str(e)}'}
            print(json.dumps(error_result), flush=True)
        except Exception as e:
            error_result = {
                'success': False,
                'error': f'Worker error: {str(e)}',
                'traceback': traceback.format_exc()
            }
            print(json.dumps(error_result), flush=True)


if __name__ == '__main__':
    main()
