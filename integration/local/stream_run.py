"""Exercise native aws-lambda-go streaming against a disposable local Runtime API."""

import argparse
import base64
import json
import subprocess
import time
import uuid
from pathlib import Path

from run import IMAGE, OUT, ROOT, prepare, run


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--reuse-builds', action='store_true')
    args = parser.parse_args()
    if not args.reuse_builds:
        prepare()
    output = OUT / 'streaming'
    output.mkdir(exist_ok=True)
    prefix = 'ptgo-stream-' + uuid.uuid4().hex[:10]
    network, capture, function = prefix, prefix + '-capture', prefix + '-lambda'
    containers = []
    report = {'scope': 'real aws-lambda-go SDK against a local Runtime API fixture; not AWS service acceptance', 'completed': False, 'cgo_enabled': False, 'runtime_architecture': 'amd64', 'binary_architectures': ['amd64', 'arm64'], 'image': IMAGE, 'checks': [], 'invocations': [], 'cleanup_errors': []}

    def check(name, passed):
        report['checks'].append({'name': name, 'passed': bool(passed)})

    def request(path, value=None):
        command = ['docker', 'exec', capture, 'curl', '--silent', '--show-error', '--fail', '--max-time', '10', '--header', 'Content-Type: application/json']
        if value is not None:
            command.extend(['--data-binary', json.dumps(value)])
        text = run(*command, 'http://localhost:4318' + path)
        return json.loads(text) if text else {}

    run('docker', 'network', 'create', '--internal', network)
    try:
        volume = f"{OUT / 'amd64'}:/var/task:ro"
        run('docker', 'run', '-d', '--name', capture, '--network', network, '--network-alias', 'capture', '--platform', 'linux/amd64', '-v', volume, '--entrypoint', '/var/task/capture', IMAGE)
        containers.append(capture)
        for _ in range(30):
            ready = subprocess.run(['docker', 'exec', capture, 'curl', '--silent', '--fail', 'http://localhost:4318/snapshot'], capture_output=True)
            if ready.returncode == 0:
                break
            time.sleep(0.1)
        else:
            raise RuntimeError('Stream Runtime API fixture did not become ready')
        command = ['docker', 'run', '-d', '--name', function, '--network', network, '--platform', 'linux/amd64', '-v', volume, '--entrypoint', '/var/task/stream-bootstrap']
        for name, value in {'AWS_LAMBDA_RUNTIME_API': 'capture:4318', 'AWS_LAMBDA_FUNCTION_NAME': 'stream-probe', 'AWS_LAMBDA_FUNCTION_MEMORY_SIZE': '256', 'AWS_REGION': 'ap-east-1', 'STREAM_FIXTURE': 'http://capture:4318', 'OTEL_EXPORTER_OTLP_ENDPOINT': 'http://capture:4318', 'OTEL_EXPORTER_OTLP_TIMEOUT': '1000', 'AWS_EC2_METADATA_DISABLED': 'true'}.items():
            command.extend(['-e', f'{name}={value}'])
        run(*command, IMAGE)
        containers.append(function)
        modes = ['success', 'success', 'read-error', 'close-error', 'panic', 'deadline', 'no-content', 'http-error', 'invalid-event', 'success']
        for index, mode in enumerate(modes):
            identifier = f'stream-{index}-{mode}'
            request('/stream/invoke', {'id': identifier, 'mode': mode, 'index': index})
            for _ in range(80):
                result = request('/stream/result/' + identifier)
                if result.get('completed'):
                    break
                time.sleep(0.1)
            else:
                raise RuntimeError(f'{identifier}: runtime response did not complete')
            report['invocations'].append({'mode': mode, 'result': result})
            if mode == 'invalid-event':
                check(f'{identifier}: invocation error before streaming', result.get('action') == 'error' and 'InvalidEventError' in result.get('body', ''))
                continue
            check(f'{identifier}: streaming SDK content type', result.get('content_type') == 'application/vnd.awslambda.http-integration-response')
            check(f'{identifier}: chunked Runtime API request', result.get('transfer_encoding') == ['chunked'])
            check(f'{identifier}: HTTP integration delimiter', result.get('delimiter_valid') is True and 'read_error' not in result)
            metadata = result.get('metadata', {})
            headers = metadata.get('headers', {})
            status = 204 if mode == 'no-content' else 400 if mode == 'http-error' else 200
            check(f'{identifier}: HTTP status and request identity', metadata.get('statusCode') == status and headers.get('x-request-id') == identifier)
            check(f'{identifier}: CORS and compression composition', headers.get('access-control-allow-origin') == 'https://app.test' and headers.get('transfer-encoding') == 'chunked' and 'content-encoding' not in headers)
            body = base64.b64decode(result.get('body_base64', '')).decode('utf-8')
            trailer = result.get('trailers', {})
            encoded_error = trailer.get('Lambda-Runtime-Function-Error-Body', [])
            failure = json.loads(base64.b64decode(encoded_error[0])) if encoded_error else None
            if mode in ('read-error', 'close-error', 'panic', 'deadline'):
                messages = {'read-error': 'stream read failure', 'close-error': 'stream close failure', 'panic': 'stream handler panic: stream read panic', 'deadline': 'context deadline exceeded'}
                check(f'{identifier}: Runtime API error trailers', failure is not None and failure.get('errorMessage') == messages[mode] and bool(trailer.get('Lambda-Runtime-Function-Error-Type')))
                check(f'{identifier}: successful prefix retained', body == ('chunk-onechunk-two' if mode == 'close-error' else 'chunk-one'))
            else:
                check(f'{identifier}: no error trailers', not encoded_error)
                if mode == 'no-content':
                    check(f'{identifier}: no-content body', body == '')
                elif mode == 'http-error':
                    check(f'{identifier}: pre-stream HTTP error', json.loads(body).get('message') == 'stream handler rejected')
                else:
                    check(f'{identifier}: gated incremental chunks', result.get('first_chunk') == 'chunk-one' and body == 'chunk-onechunk-two')
        snapshot = request('/snapshot')
        (output / 'otlp.json').write_text(json.dumps(snapshot, indent=2) + '\n', encoding='utf-8', newline='\n')
        logs = run('docker', 'logs', function)
        (output / 'lambda.log').write_text(logs, encoding='utf-8', newline='\n')
        records = []
        for line in logs.splitlines():
            if line.startswith('{'):
                records.append(json.loads(line))
        spans = [span for batch in snapshot.get('batches', []) for resource in batch.get('resourceSpans', []) for scope in resource.get('scopeSpans', []) for span in scope.get('spans', [])]
        for index, item in enumerate(report['invocations']):
            mode, result = item['mode'], item['result']
            identifier = result['id']
            expected_trace = '6aaab31e' + f'{index + 1:024x}'
            complete = [record for record in records if record.get('message') == 'stream complete' and record.get('function_request_id') == identifier]
            closes = 0 if mode in ('no-content', 'http-error', 'invalid-event') else 1
            check(f'{identifier}: body closed before invocation cleanup', len(complete) == 1 and complete[0].get('body_closes') == closes)
            traced = [span for span in spans if base64.b64decode(span['traceId']).hex() == expected_trace and span.get('name') == '## stream-invocation']
            check(f'{identifier}: correlated span and log', len(traced) == 1 and len(complete) == 1 and complete[0].get('trace_id') == expected_trace and complete[0].get('span_id') == base64.b64decode(traced[0]['spanId']).hex())
            error = mode in ('read-error', 'close-error', 'panic', 'deadline', 'invalid-event')
            check(f'{identifier}: OTel terminal error state', len(traced) == 1 and (traced[0].get('status', {}).get('code') == 'STATUS_CODE_ERROR') == error)
        check('warm execution continues after stream failures', len(report['invocations']) == len(modes) and report['invocations'][-1]['result'].get('metadata', {}).get('statusCode') == 200)
        report['completed'] = True
    finally:
        if function in containers:
            logs = subprocess.run(['docker', 'logs', function], capture_output=True, text=True, encoding='utf-8')
            (output / 'lambda.log').write_text(logs.stdout + '\n' + logs.stderr, encoding='utf-8', newline='\n')
        for container in reversed(containers):
            result = subprocess.run(['docker', 'rm', '-f', container], capture_output=True, text=True)
            if result.returncode:
                report['cleanup_errors'].append(result.stderr.strip())
        result = subprocess.run(['docker', 'network', 'rm', network], capture_output=True, text=True)
        if result.returncode:
            report['cleanup_errors'].append(result.stderr.strip())
        report['passed'] = report['completed'] and bool(report['checks']) and all(check['passed'] for check in report['checks']) and not report['cleanup_errors'] and len(report['invocations']) == 10
        (output / 'report.json').write_text(json.dumps(report, indent=2) + '\n', encoding='utf-8', newline='\n')
    if not report['passed']:
        raise RuntimeError(f"Stream acceptance failures: {[check['name'] for check in report['checks'] if not check['passed']]}; cleanup: {report['cleanup_errors']}")
    report['assertion_count'] = len(report['checks'])
    (ROOT / 'docs/STREAMING_ACCEPTANCE.json').write_text(json.dumps(report, indent=2) + '\n', encoding='utf-8', newline='\n')
    print(f"Streaming: {len(report['checks'])}/{len(report['checks'])} checks passed; real Go Lambda SDK, local Runtime API fixture")


if __name__ == '__main__':
    main()
