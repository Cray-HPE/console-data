#
# MIT License
#
# (C) Copyright [2022] Hewlett Packard Enterprise Development LP
#
# Permission is hereby granted, free of charge, to any person obtaining a
# copy of this software and associated documentation files (the "Software"),
# to deal in the Software without restriction, including without limitation
# the rights to use, copy, modify, merge, publish, distribute, sublicense,
# and/or sell copies of the Software, and to permit persons to whom the
# Software is furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included
# in all copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
# THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR
# OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
# ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
# OTHER DEALINGS IN THE SOFTWARE.
#

FROM artifactory.algol60.net/csm-docker/stable/registry.suse.com/suse/sle15:15.3 as build

FROM artifactory.algol60.net/csm-docker/stable/registry.suse.com/suse/sle15:15.3 as base


ARG SLES_REPO_USERNAME
ARG SLES_REPO_PASSWORD
ARG SLES_MIRROR="https://${SLES_REPO_USERNAME}:${SLES_REPO_PASSWORD}@artifactory.algol60.net/artifactory/sles-mirror"
ARG ARCH=x86_64
ARG MUNGE_UID=600
ARG MUNGE_GID=600
ARG MUNGE_NAME=munge

# attach volumes now so chmod commands impact these dirs
VOLUME /var/run/munge /etc/munge

# Steps in the following order:
# - create munge group and user with specified ID's (that can be used in charts)
# - install munge (uses created user/group)
# - clean up dir ownership and permissions

RUN \
  addgroup -g ${MUNGE_GID} ${MUNGE_NAME} && adduser -u ${MUNGE_UID} -G ${MUNGE_GID} ${MUNGE_NAME} && \
  zypper --non-interactive rr --all && \
  zypper --non-interactive ar ${SLES_MIRROR}/Products/SLE-Module-Basesystem/15-SP3/${ARCH}/product?auth=basic sles15sp3-Module-Basesystem-product && \
  zypper --non-interactive ar ${SLES_MIRROR}/Updates/SLE-Module-Basesystem/15-SP3/${ARCH}/update?auth=basic sles15sp3-Module-Basesystem-update && \
  zypper --non-interactive ar ${SLES_MIRROR}/Products/SLE-Module-HPC/15-SP3/${ARCH}/product?auth=basic sles15sp3-Module-HPC-product && \
  zypper --non-interactive ar ${SLES_MIRROR}/Updates/SLE-Module-HPC/15-SP3/${ARCH}/update?auth=basic sles15sp3-Module-HPC-update && \
  zypper update -y && \
  zypper install -y munge && \
  zypper clean -a && zypper --non-interactive rr --all && rm -f /etc/zypp/repos.d/* && rm -Rf /root/.zypp && \
  chmod 3777 /run/munge /var/run/munge && \
  chmod 0700 /etc/munge /var/lib/munge /var/log/munge

# Switch to the munge user/group to run
USER ${MUNGE_UID}:${MUNGE_GID}

# enter the 'munged' process in the foreground
# NOTE: -f forces to start even with warnings present - remove when able to
ENTRYPOINT ["/usr/sbin/munged", "-F", "-f"]
