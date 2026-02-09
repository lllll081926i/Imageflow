#!/usr/bin/env python3
"""
Python Worker Process for ImageFlow
This script runs as a long-lived process that can execute multiple image processing tasks.
It reads commands from stdin and writes results to stdout using JSON.
"""

import sys
import os
import json
import importlib
import traceback
import gc
import time
from typing import Dict, Any


SCRIPTS_DIR = os.path.dirname(os.path.abspath(__file__))
if SCRIPTS_DIR not in sys.path:
    sys.path.insert(0, SCRIPTS_DIR)


_CONVERTER_MODULE = None
_PRELOAD_DONE = False


def _get_converter_module():
    global _CONVERTER_MODULE
    if _CONVERTER_MODULE is None:
        module = importlib.import_module("converter")
        if not hasattr(module, "process"):
            raise RuntimeError("converter module missing process()")
        _CONVERTER_MODULE = module
    return _CONVERTER_MODULE


def _run_converter_in_process(input_data: Dict[str, Any]) -> Dict[str, Any]:
    try:
        module = _get_converter_module()
        result = module.process(input_data)
        if isinstance(result, dict):
            return result
        return {"success": False, "error": "converter returned non-object result"}
    except Exception as e:
        return {
            "success": False,
            "error": f"converter failed: {e}",
            "traceback": traceback.format_exc()[-4000:],
        }


def _preload_modules() -> None:
    global _PRELOAD_DONE
    if _PRELOAD_DONE:
        return
    start = time.perf_counter()
    try:
        _get_converter_module()
        try:
            from PIL import Image  # type: ignore
            Image.init()
        except Exception:
            pass
        if os.getenv("IMAGEFLOW_PROFILE") == "1":
            try:
                from PIL import features  # type: ignore
                checks = ["jpg", "jpg_2000", "png", "webp", "avif", "tiff", "libjpeg_turbo"]
                for key in checks:
                    ok = features.check(key)
                    print(f"INFO PIL feature {key}={ok}", file=sys.stderr)
            except Exception as e:
                print(f"WARNING PIL feature check failed: {e}", file=sys.stderr)
        elapsed = time.perf_counter() - start
        print(f"INFO Worker preload complete in {elapsed:.2f}s", file=sys.stderr)
    except Exception as e:
        elapsed = time.perf_counter() - start
        print(f"WARNING Worker preload failed in {elapsed:.2f}s: {e}", file=sys.stderr)
    _PRELOAD_DONE = True


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
            return _run_converter_in_process(input_data)

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

    _preload_modules()

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
