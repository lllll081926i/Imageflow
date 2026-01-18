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
from typing import Dict, Any


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
    # Unbuffer stdout to ensure immediate output
    sys.stdout.reconfigure(line_buffering=True)

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
