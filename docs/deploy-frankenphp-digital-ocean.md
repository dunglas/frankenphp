# Deploying FrankenPHP on DigitalOcean


## Prerequisites

1.  **DigitalOcean Account:** You'll need a DigitalOcean account. If you don't have one, you can sign up [here](https://www.digitalocean.com/).
2.  **SSH Key:** Ensure you have an SSH key configured on your local machine. You can follow [DigitalOcean's SSH key setup guide](https://www.digitalocean.com/docs/ssh/create-ssh-keys/) for this.
3.  **Domain Name (Optional):** If you have a domain name, you can configure it to point to your DigitalOcean droplet for a custom URL.

## Step 1: Create a DigitalOcean Droplet

1.  Log in to your DigitalOcean account.
2.  Click the "Create Droplet" button.
3.  Choose an image, plan, data center region, and configure additional options.
4.  In the "Authentication" section, select your SSH key.
5.  Give your droplet a hostname, and click "Create Droplet."

## Step 2: SSH into Your Droplet

Once your droplet is created, you can SSH into it using your terminal:

`ssh your_username@your_droplet_ip`

##  Step 3: Install FrankenPHP

Now, you'll need to install FrankenPHP on your DigitalOcean droplet. You can use Docker for this purpose:

`docker run -v $PWD:/app/public -p 80:80 -p 443:443 dunglas/frankenphp`

## Step 4: Configure Your Domain (Optional)

1.  Log in to your domain registrar's website.
2.  Update the DNS settings to point to your DigitalOcean droplet's IP address.
3.  Wait for DNS propagation, which can take some time.

## Step 5: Access FrankenPHP

Once FrankenPHP is up and running on your DigitalOcean droplet, you can access it by opening a web browser and navigating to your droplet's IP address or domain name (if configured).

*   For IP address: `http://your_droplet_ip`
*   For domain name: `http://your_domain_name`

## Step 6: Enjoy FrankenPHP

You have successfully deployed FrankenPHP on DigitalOcean. You can now use its features to enhance your PHP applications. Remember to secure your server and FrankenPHP installation, and regularly update both for the best performance and security.

For more detailed configurations and advanced usage, refer to the [FrankenPHP documentation](https://frankenphp.dev/).

Enjoy using FrankenPHP on DigitalOcean!
