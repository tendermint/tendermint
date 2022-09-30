---
order: 1
title: Method
---

The (first iteration of the) QA process as described [here][releases] is applied to version v0.34.x
in order to have a set of results acting as benckmarking baseline.
This baseline is then compared with results obtained in later versions.

Out of the testnet-based test cases described in [releases][releases] we are focusing on two of them:
_200 Node Test_, and _Rotating Nodes Test_.

[releases]: https://github.com/tendermint/tendermint/blob/v0.37.x/RELEASES.md#large-scale-testnets

# 200 Node Testnet

## Running the test

This section explains how the tests were carried out for reproducibility purposes.

### Infrastructure Requirements

* An account at Digital Ocean (DO), with a high droplet limit (>202)
* The machine to orchestrate the tests should have the following installed:
    * A clone of the [testnet repository][testnet-repo]
        * This repository contains all the scripts mentioned in the reminder of this section
    * [Digital Ocean CLI][doctl]
    * [Terraform CLI][Terraform]
    * [Ansible CLI][Ansible]

### Steps

1. Follow steps 1-4 of the `README.md` at the top of the testnet repository to configure Terraform, and `doctl`.
2. Copy file `testnet200.toml` onto `testnet200.toml` (do NOT commit this change)
3. Set variable `VERSION_TAG` to the git hash that is to be tested.
4. Follow steps 5-10 of the `README.md` to configure and start the 200 node testnet
    * WARNING: Do NOT forget to run `make terraform-destroy` as soon as you are done with the tests
5. As a sanity check, connect to the Prometheus node and check the graph for the `tendermint_consensus_height` metric.
   All nodes should be increasing their heights.
6. `ssh` into the `testnet-load-runner` and run script XXXXX
    * This will take about 40 mins to run
    * It is running 90-seconds-long experiments in a loop with different loads
7. Run `make retrieve-data` to gather all relevant data from the testnet into the orchestrating machine
8. Verify that the data was collected without errors
    * all Tendermint nodes' blockstore DB
    * the Prometheus database from the Prometheus node
9. **Run `make terraform-destroy`**

[testnet-repo]: https://github.com/interchainio/tendermint-testnet
[Ansible]: https://docs.ansible.com/ansible/latest/index.html
[Terraform]: https://www.terraform.io/docs
[doctl]: https://docs.digitalocean.com/reference/doctl/how-to/install/

## Result Extraction

The method for extracting the results described here is highly manual (and exploratory) at this stage.
The Core team should improve it at every iteration to increase the amount of automation.

### Requirements

* Matlab or Octave
* Prometheus server installed
* blockstore DB of one of the full nodes in the testnet
* Prometheus DB

### Steps

1. Unzip the blockstore into a directory
2. Extract the latency report and the raw latencies for all the experiments. Run these commands from the directory containing the blockstore
    * `go run github.com/tendermint/tendermint/test/loadtime/cmd/report@5fe1a7241 --database-type goleveldb --data-dir ./ > results/report.txt`
    * `go run github.com/tendermint/tendermint/test/loadtime/cmd/report@5fe1a7241 --database-type goleveldb --data-dir ./ --csv results/raw.csv`
3. File `report.txt` contains an unordered list of experiments with varying concurrent connections and transaction rate
    * Create files `report04.txt`, `report08.txt`, `report16.txt` and, for each experiment in file `report.txt`,
      copy its related lines to the filename that matches the number of connections.
    * Sort the experiments in `report04.txt` in ascending tx rate order. Likewise for `report08.txt` and `report16.txt`.
4. Generate file `report_tabbed.txt` by showing the contents `report04.txt`, `report08.txt`, `report16.txt` side by side
   * This effectively creates a table where rows are a particular tx rate and columns are a particular number of websocket connections.
5. Extract the raw latencies from file `raw.csv` using the following bash loop. This creates a `.csv` file and a `.dat` file per experiment.
   The format of the `.dat` files is amenable to loading them as matrices in Octave

    ```bash
    uuids=($(cat report04.txt report08.txt report16.txt | grep '^Experiment ID: ' | awk '{ print $3 }')) 
    c=1
    for i in 04 08 16; do
      for j in 0020 0200 0400 0800 1200; do
        echo $i $j $c "$uuids[$c]"
        filename=c${i}_r${j}
        grep $uuids[$c] raw.csv > ${filename}.csv
        cat ${filename}.csv | tr , ' ' | awk '{ print $3, $2 }' > ${filename}.dat 
        c=$(expr $c + 1)
      done
    done
    ```

6. Enter Octave
7. Load all `.dat` files generated in step 5 into matrices using this Octave code snippet

    ```octave
    conns =  { "04"; "08"; "16" };
    rates =  { "0020"; "0200"; "0400"; "0800"; "1200" };
    for i = 1:length(conns)
      for j = 1:length(rates)
        filename = strcat("c", conns{i}, "_r", rates{j}, ".dat");
        load("-ascii", filename);
      endfor
    endfor
    ```

8. Set variable release to the current release undergoing QA

    ```octave
    release = "v0.34.x";
    ```

9. Generate a plot with all (or some) experiments, where the X axis is the experiment time,
   and the y axis is the latency of transactions.
   The following snippet plots all experiments.

    ```octave
    legends = {};
    hold off;
    for i = 1:length(conns)
      for j = 1:length(rates)
        data_name = strcat("c", conns{i}, "_r", rates{j});
        l = strcat("c=", conns{i}, " r=", rates{j});
        m = eval(data_name); plot((m(:,1) - min(m(:,1))) / 1e+9, m(:,2) / 1e+9, ".");
        hold on;
        legends(1, end+1) = l;
      endfor
    endfor
    legend(legends, "location", "northeastoutside");
    xlabel("experiment time (s)");
    ylabel("latency (s)");
    axis([0, 100, 0, 15], "tic");
    t = sprintf("200-node testnet - %s", release);
    title(t);
    ```

10. Use Octave's GUI menu to save the plot (e.g. as `.svg`)

11. Repeat steps 9 and 10 to obtain as many plots as deemed necessary.

### Extracting Prometheus Metrics

1. Stop the prometheus server if it is running as a service (e.g. a `systemd` unit).
2. Unzip the prometheus database retrieved from the testnet, and move it to replace the 
   local prometheus database.
3. Start the prometheus server and make sure no error logs appear at start up.
4. Introduce the metrics you want to gather or plot.

# Rotating Node Testnet

TODO