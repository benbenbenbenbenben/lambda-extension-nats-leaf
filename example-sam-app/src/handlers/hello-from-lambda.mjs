import * as fs from "node:fs/promises"
/**
 * A Lambda function that returns a static string
 */
export const helloFromLambdaHandler = async () => {

    // Wait a second
    await new Promise(resolve => setTimeout(resolve, 1000));

    const stat = await fs.stat("/tmp/nats-extension.lock");
    if (stat.isFile()) { // Ensure the file exists
        const message = `Hello from Lambda!, the file /tmp/nats-extension.lock was last modified at ${stat.mtime}`;

        // All log statements are written to CloudWatch
        console.info(`${message}`);
        return message;
    }
    return "Hello from Lambda!, the file /tmp/nats-extension.lock does not exist";
}
